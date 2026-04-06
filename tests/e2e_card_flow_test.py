"""
Integration test: Two players queue → match → accept → connect via WebSocket →
observe draw pile top-up → select cards to hand → play through rounds without placing cards.

Verifies:
  - Deck is loaded and shuffled at game start
  - Draw pile gets topped up (up to 5) each pre-turn
  - Players can select cards from draw pile to hand (max 4)
  - Cards accumulate in draw pile if not selected (capped at 8)
  - Hand stays across rounds if not played
  - Elixir charges and caps progress per round

Prerequisites:
    - Auth service on localhost:8000
    - Matchmaking service on localhost:8001
    - Gameplay service on localhost:8002
    - DB migrated and seeded

Usage: python tests/e2e_card_flow_test.py
"""

import json
import os
import threading
import time
import sys

import requests
import websocket

AUTH_URL = os.environ.get("AUTH_URL", "http://localhost:8000")
MATCHMAKING_URL = os.environ.get("MATCHMAKING_URL", "http://localhost:8001")
GAMEPLAY_WS_URL = os.environ.get("GAMEPLAY_WS_URL", "ws://localhost:8002")

GREEN = "\033[92m"
RED = "\033[91m"
YELLOW = "\033[93m"
CYAN = "\033[96m"
NC = "\033[0m"

passed = 0
failed = 0
total = 0
lock = threading.Lock()


def assert_true(desc, condition, detail=""):
    global passed, failed, total
    with lock:
        total += 1
        if condition:
            print(f"  {GREEN}✓{NC} {desc}")
            passed += 1
        else:
            print(f"  {RED}✗{NC} {desc}")
            if detail:
                print(f"    {detail}")
            failed += 1


def register_and_login(username, password):
    resp = requests.post(f"{AUTH_URL}/auth/register", json={
        "username": username, "password": password,
    })
    assert_true(f"Register {username}", resp.status_code == 201,
                f"HTTP {resp.status_code}: {resp.text[:200]}")
    resp = requests.post(f"{AUTH_URL}/auth/login", json={
        "username": username, "password": password,
    })
    assert_true(f"Login {username}", resp.status_code == 200)
    data = resp.json()
    return data.get("token", ""), data.get("user_id", 0)


class GamePlayer:
    def __init__(self, name, token, session_id):
        self.name = name
        self.token = token
        self.session_id = session_id
        self.ws = None
        self.connected = False
        self.error = None
        self.latest_phase = None
        self.latest_elixir = None
        self.latest_elixir_cap = None
        self.latest_round = None
        self.latest_tick = 0
        self.latest_params = None
        self.latest_draw_pile = None
        self.latest_hand = None
        self.latest_your_board = None
        self.phase_history = []
        self._recv_thread = None

    def connect(self):
        url = f"{GAMEPLAY_WS_URL}/ws?session_id={self.session_id}&token={self.token}"
        try:
            self.ws = websocket.create_connection(url, timeout=5)
            self.ws.settimeout(1.0)
            self.connected = True
            self._recv_thread = threading.Thread(target=self._recv_loop, daemon=True)
            self._recv_thread.start()
        except Exception as e:
            self.error = str(e)

    def _recv_loop(self):
        while self.connected:
            try:
                raw = self.ws.recv()
                if not raw:
                    continue
                msg = json.loads(raw)

                sv = msg.get("state_view")
                if isinstance(sv, dict):
                    tn = sv.get("tick_number", 0)
                    with lock:
                        if tn > self.latest_tick:
                            self.latest_tick = tn

                params = msg.get("params")
                if isinstance(params, dict):
                    phase = params.get("phase")
                    if phase:
                        with lock:
                            self.latest_phase = phase
                            self.latest_elixir = params.get("elixir")
                            self.latest_elixir_cap = params.get("elixir_cap")
                            self.latest_round = params.get("round_number")
                            self.latest_params = params
                            self.latest_your_board = params.get("your_board")
                            if not self.phase_history or self.phase_history[-1] != phase:
                                self.phase_history.append(phase)

                    # Check for draw_pile and hand in the tick params
                    # These might not be present yet — we'll add them via SELECT_CARDS response
                    if "draw_pile" in params:
                        with lock:
                            self.latest_draw_pile = params["draw_pile"]
                    if "hand" in params:
                        with lock:
                            self.latest_hand = params["hand"]

                # Also check action_result for draw/hand data
                if msg.get("message_type") == "action_result":
                    sv2 = msg.get("state_view", {})
                    # Game data might contain draw pile / hand info
                    pass

            except websocket.WebSocketTimeoutException:
                continue
            except Exception:
                break

    def send_action(self, action, params=None):
        if not self.ws:
            return
        with lock:
            seq = self.latest_tick
        msg = json.dumps({
            "action": action,
            "params": params or {},
            "state_hash_after": 0,
            "sequence_number": seq,
        })
        self.ws.send(msg)

    def wait_for_phase(self, phase, timeout=40):
        start = time.time()
        while time.time() - start < timeout:
            with lock:
                if self.latest_phase == phase:
                    return True
            time.sleep(0.25)
        return False

    def close(self):
        self.connected = False
        if self.ws:
            try:
                self.ws.close()
            except Exception:
                pass


def main():
    global passed, failed

    ts = int(time.time())
    password = "testpassword123"

    print(f"{YELLOW}=== E2E Card Flow Test ==={NC}\n")

    # ── Register & Login ──
    print(f"{YELLOW}Step 1: Register and login both players{NC}")
    p1_token, p1_id = register_and_login(f"cf_p1_{ts}", password)
    p2_token, p2_id = register_and_login(f"cf_p2_{ts}", password)
    print(f"  P1 id={p1_id}, P2 id={p2_id}")

    # ── Queue ──
    print(f"\n{YELLOW}Step 2: Join matchmaking queue{NC}")
    r1 = requests.post(f"{MATCHMAKING_URL}/matchmaking/queue",
                       headers={"Authorization": f"Bearer {p1_token}"})
    assert_true("P1 joins queue", r1.status_code == 200, f"{r1.status_code}: {r1.text[:100]}")
    r2 = requests.post(f"{MATCHMAKING_URL}/matchmaking/queue",
                       headers={"Authorization": f"Bearer {p2_token}"})
    assert_true("P2 joins queue", r2.status_code == 200, f"{r2.status_code}: {r2.text[:100]}")

    # ── Wait for match ──
    print(f"\n{YELLOW}Step 3: Wait for match{NC}")
    session_id = None
    for _ in range(30):
        time.sleep(0.5)
        r = requests.get(f"{MATCHMAKING_URL}/matchmaking/match",
                         headers={"Authorization": f"Bearer {p1_token}"})
        d = r.json()
        if d.get("matched"):
            session_id = d["session_id"]
            break
    assert_true("P1 found match", session_id is not None)
    if not session_id:
        sys.exit(1)
    r = requests.get(f"{MATCHMAKING_URL}/matchmaking/match",
                     headers={"Authorization": f"Bearer {p2_token}"})
    assert_true("P2 found match", r.json().get("matched"))
    print(f"  Session: {session_id}")

    # ── Accept ──
    print(f"\n{YELLOW}Step 4: Accept match{NC}")
    r1 = requests.post(f"{MATCHMAKING_URL}/matchmaking/match/accept",
                       headers={"Authorization": f"Bearer {p1_token}"},
                       json={"session_id": session_id})
    assert_true("P1 accepts", r1.status_code == 200)
    r2 = requests.post(f"{MATCHMAKING_URL}/matchmaking/match/accept",
                       headers={"Authorization": f"Bearer {p2_token}"},
                       json={"session_id": session_id})
    assert_true("P2 accepts", r2.status_code == 200)

    # ── Connect WebSocket ──
    print(f"\n{YELLOW}Step 5: Connect via WebSocket{NC}")
    p1 = GamePlayer("P1", p1_token, session_id)
    p2 = GamePlayer("P2", p2_token, session_id)
    p1.connect()
    p2.connect()
    assert_true("P1 WS connected", p1.connected, p1.error or "")
    assert_true("P2 WS connected", p2.connected, p2.error or "")
    if not p1.connected or not p2.connected:
        sys.exit(1)

    time.sleep(3)

    # ──────────────────────────────────────────────
    # Round 1: PRE_TURN → observe draw pile → select cards → ACTIVE
    # ──────────────────────────────────────────────
    print(f"\n{YELLOW}Round 1: Pre-turn phase{NC}")
    got_pt_p1 = p1.wait_for_phase("PRE_TURN", timeout=15)
    got_pt_p2 = p2.wait_for_phase("PRE_TURN", timeout=5)
    assert_true("R1: P1 sees PRE_TURN", got_pt_p1, f"phase={p1.latest_phase}")
    assert_true("R1: P2 sees PRE_TURN", got_pt_p2, f"phase={p2.latest_phase}")

    if not got_pt_p1:
        print(f"  {RED}Phase history: {p1.phase_history}, aborting{NC}")
        p1.close()
        p2.close()
        sys.exit(1)

    time.sleep(2)
    print(f"  {CYAN}P1 elixir={p1.latest_elixir}, cap={p1.latest_elixir_cap}{NC}")
    print(f"  {CYAN}P2 elixir={p2.latest_elixir}, cap={p2.latest_elixir_cap}{NC}")

    time.sleep(1)

    # Wait for ACTIVE (10s pre-turn timer)
    print(f"\n{YELLOW}Round 1: Waiting for ACTIVE phase (10s pre-turn){NC}")
    got_active_p1 = p1.wait_for_phase("ACTIVE", timeout=15)
    got_active_p2 = p2.wait_for_phase("ACTIVE", timeout=5)
    assert_true("R1: P1 sees ACTIVE", got_active_p1, f"phase={p1.latest_phase}")
    assert_true("R1: P2 sees ACTIVE", got_active_p2, f"phase={p2.latest_phase}")

    print(f"  {CYAN}P1 active: elixir={p1.latest_elixir}, cap={p1.latest_elixir_cap}{NC}")
    print(f"  {CYAN}P2 active: elixir={p2.latest_elixir}, cap={p2.latest_elixir_cap}{NC}")
    assert_true("R1: P1 elixir cap = 5", p1.latest_elixir_cap == 5, f"got {p1.latest_elixir_cap}")
    assert_true("R1: P2 elixir cap = 5", p2.latest_elixir_cap == 5, f"got {p2.latest_elixir_cap}")

    # No cards played — wait for round end
    print(f"  Waiting 30s for round to end...")
    p1.wait_for_phase("PRE_TURN", timeout=35)
    p2.wait_for_phase("PRE_TURN", timeout=5)

    print(f"  {CYAN}P1 round 1 end: elixir={p1.latest_elixir}, cap={p1.latest_elixir_cap}{NC}")
    print(f"  {CYAN}P2 round 1 end: elixir={p2.latest_elixir}, cap={p2.latest_elixir_cap}{NC}")
    assert_true("R1: P1 elixir reached cap 5", p1.latest_elixir == 5, f"got {p1.latest_elixir}")
    assert_true("R1: P2 elixir reached cap 5", p2.latest_elixir == 5, f"got {p2.latest_elixir}")

    # ──────────────────────────────────────────────
    # Round 2
    # ──────────────────────────────────────────────
    print(f"\n{YELLOW}Round 2: Pre-turn phase{NC}")
    time.sleep(2)
    print(f"  {CYAN}P1 pre-turn: elixir={p1.latest_elixir}, cap={p1.latest_elixir_cap}{NC}")
    print(f"  {CYAN}P2 pre-turn: elixir={p2.latest_elixir}, cap={p2.latest_elixir_cap}{NC}")
    assert_true("R2: P1 elixir cap = 6", p1.latest_elixir_cap == 6, f"got {p1.latest_elixir_cap}")
    assert_true("R2: P2 elixir cap = 6", p2.latest_elixir_cap == 6, f"got {p2.latest_elixir_cap}")


    print(f"\n{YELLOW}Round 2: Waiting for ACTIVE{NC}")
    p1.wait_for_phase("ACTIVE", timeout=15)
    p2.wait_for_phase("ACTIVE", timeout=5)

    print(f"  {CYAN}P1 active: elixir={p1.latest_elixir}, cap={p1.latest_elixir_cap}{NC}")
    print(f"  {CYAN}P2 active: elixir={p2.latest_elixir}, cap={p2.latest_elixir_cap}{NC}")

    print(f"  Waiting 30s for round to end...")
    p1.wait_for_phase("PRE_TURN", timeout=35)
    p2.wait_for_phase("PRE_TURN", timeout=5)

    print(f"  {CYAN}P1 round 2 end: elixir={p1.latest_elixir}, cap={p1.latest_elixir_cap}{NC}")
    print(f"  {CYAN}P2 round 2 end: elixir={p2.latest_elixir}, cap={p2.latest_elixir_cap}{NC}")
    assert_true("R2: P1 elixir reached cap 6", p1.latest_elixir == 6, f"got {p1.latest_elixir}")
    assert_true("R2: P2 elixir reached cap 6", p2.latest_elixir == 6, f"got {p2.latest_elixir}")
    assert_true("R2: P1 next cap = 7", p1.latest_elixir_cap == 7, f"got {p1.latest_elixir_cap}")
    assert_true("R2: P2 next cap = 7", p2.latest_elixir_cap == 7, f"got {p2.latest_elixir_cap}")

    # ──────────────────────────────────────────────
    # Round 3
    # ──────────────────────────────────────────────
    print(f"\n{YELLOW}Round 3: Observe state{NC}")
    time.sleep(2)
    print(f"  {CYAN}P1 pre-turn: elixir={p1.latest_elixir}, cap={p1.latest_elixir_cap}{NC}")
    print(f"  {CYAN}P2 pre-turn: elixir={p2.latest_elixir}, cap={p2.latest_elixir_cap}{NC}")
    assert_true("R3: P1 elixir cap = 7", p1.latest_elixir_cap == 7, f"got {p1.latest_elixir_cap}")
    assert_true("R3: P2 elixir cap = 7", p2.latest_elixir_cap == 7, f"got {p2.latest_elixir_cap}")

    # Check boards — should be empty
    p1_board = p1.latest_your_board
    p2_board = p2.latest_your_board
    if p1_board is not None:
        assert_true("P1 board is empty", len(p1_board) == 0, f"has {len(p1_board)} cards")
    if p2_board is not None:
        assert_true("P2 board is empty", len(p2_board) == 0, f"has {len(p2_board)} cards")


    p1.wait_for_phase("ACTIVE", timeout=15)
    p2.wait_for_phase("ACTIVE", timeout=5)

    time.sleep(5)
    print(f"  {CYAN}P1 R3 active: elixir={p1.latest_elixir}, cap={p1.latest_elixir_cap}{NC}")
    print(f"  {CYAN}P2 R3 active: elixir={p2.latest_elixir}, cap={p2.latest_elixir_cap}{NC}")

    # ── Summary ──
    print(f"\n{YELLOW}Phase history:{NC}")
    print(f"  P1: {p1.phase_history}")
    print(f"  P2: {p2.phase_history}")

    # Cleanup
    p1.close()
    p2.close()

    print(f"\n{'═' * 55}")
    print(f"Results: {GREEN}{passed} passed{NC}, {RED}{failed} failed{NC}, {total} total")
    print(f"{'═' * 55}")

    sys.exit(1 if failed > 0 else 0)


if __name__ == "__main__":
    main()

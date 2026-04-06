"""
Integration test: Two players queue → match → accept → connect via WebSocket →
play 5 rounds (no cards placed) → verify elixir caps progress.

Prerequisites:
    - Auth service on localhost:8000
    - Matchmaking service on localhost:8001
    - Gameplay service on localhost:8002
    - DB migrated and seeded

Usage: python tests/e2e_game_loop_test.py
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
                f"HTTP {resp.status_code}: {resp.text[:100]}")
    resp = requests.post(f"{AUTH_URL}/auth/login", json={
        "username": username, "password": password,
    })
    assert_true(f"Login {username}", resp.status_code == 200)
    data = resp.json()
    return data.get("token", ""), data.get("user_id", 0)


class GamePlayer:
    """Represents a player connected to the game via WebSocket."""

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
        self.latest_params = None
        self.latest_tick = 0
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
                params = msg.get("params")
                # Track tick number from state_view
                sv = msg.get("state_view")
                if isinstance(sv, dict):
                    tn = sv.get("tick_number", 0)
                    with lock:
                        if tn > self.latest_tick:
                            self.latest_tick = tn

                if isinstance(params, dict):
                    phase = params.get("phase")
                    if phase:
                        with lock:
                            self.latest_phase = phase
                            self.latest_elixir = params.get("elixir")
                            self.latest_elixir_cap = params.get("elixir_cap")
                            self.latest_round = params.get("round_number")
                            self.latest_params = params
                            if not self.phase_history or self.phase_history[-1] != phase:
                                self.phase_history.append(phase)
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

    print(f"{YELLOW}=== E2E Game Loop Test (5 Rounds, No Cards) ==={NC}\n")

    # ── Register & Login ──
    print(f"{YELLOW}Step 1: Register and login both players{NC}")
    p1_token, p1_id = register_and_login(f"gl_p1_{ts}", password)
    p2_token, p2_id = register_and_login(f"gl_p2_{ts}", password)
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
        print(f"  {RED}No match found, aborting{NC}")
        sys.exit(1)
    # P2 also checks
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

    # Wait for initial state to arrive
    time.sleep(3)

    # ── Play 5 rounds ──
    for round_num in range(1, 6):
        print(f"\n{YELLOW}Round {round_num}{NC}")

        # Wait for PRE_TURN
        got_pt_p1 = p1.wait_for_phase("PRE_TURN", timeout=40)
        got_pt_p2 = p2.wait_for_phase("PRE_TURN", timeout=5)
        assert_true(f"R{round_num}: P1 sees PRE_TURN", got_pt_p1, f"phase={p1.latest_phase}")
        assert_true(f"R{round_num}: P2 sees PRE_TURN", got_pt_p2, f"phase={p2.latest_phase}")

        if not got_pt_p1:
            print(f"  {RED}Stuck — P1 phase history: {p1.phase_history}{NC}")
            break

        # Record elixir at pre-turn
        print(f"  {CYAN}P1 pre-turn: elixir={p1.latest_elixir}, cap={p1.latest_elixir_cap}{NC}")
        print(f"  {CYAN}P2 pre-turn: elixir={p2.latest_elixir}, cap={p2.latest_elixir_cap}{NC}")

        # Both skip selection
        # no selection needed
        # no selection needed

        # Wait for ACTIVE (pre-turn lasts 10 seconds)
        got_active_p1 = p1.wait_for_phase("ACTIVE", timeout=15)
        got_active_p2 = p2.wait_for_phase("ACTIVE", timeout=5)
        assert_true(f"R{round_num}: P1 sees ACTIVE", got_active_p1, f"phase={p1.latest_phase}")
        assert_true(f"R{round_num}: P2 sees ACTIVE", got_active_p2, f"phase={p2.latest_phase}")

        if not got_active_p1:
            print(f"  {RED}Stuck — P1 phase history: {p1.phase_history}{NC}")
            break

        print(f"  {CYAN}P1 active: elixir={p1.latest_elixir}, cap={p1.latest_elixir_cap}{NC}")
        print(f"  {CYAN}P2 active: elixir={p2.latest_elixir}, cap={p2.latest_elixir_cap}{NC}")

        # Verify elixir cap matches expected for this round
        expected_cap = min(round_num + 4, 8)  # round 1=5, round 2=6, ..., round 4+=8
        assert_true(f"R{round_num}: P1 elixir cap = {expected_cap}", p1.latest_elixir_cap == expected_cap,
                    f"got {p1.latest_elixir_cap}")
        assert_true(f"R{round_num}: P2 elixir cap = {expected_cap}", p2.latest_elixir_cap == expected_cap,
                    f"got {p2.latest_elixir_cap}")

        # Wait for round to end (30s)
        if round_num < 5:
            print(f"  Waiting 30s for round to end...")
            got_next_p1 = p1.wait_for_phase("PRE_TURN", timeout=35)
            got_next_p2 = p2.wait_for_phase("PRE_TURN", timeout=5)
            assert_true(f"R{round_num}: P1 round ended → PRE_TURN", got_next_p1,
                        f"phase={p1.latest_phase}")
            assert_true(f"R{round_num}: P2 round ended → PRE_TURN", got_next_p2,
                        f"phase={p2.latest_phase}")

            print(f"  {CYAN}P1 round end: elixir={p1.latest_elixir}, cap={p1.latest_elixir_cap}{NC}")
            print(f"  {CYAN}P2 round end: elixir={p2.latest_elixir}, cap={p2.latest_elixir_cap}{NC}")

            assert_true(f"R{round_num}: P1 elixir reached cap ({expected_cap})",
                        p1.latest_elixir == expected_cap, f"got {p1.latest_elixir}")
            assert_true(f"R{round_num}: P2 elixir reached cap ({expected_cap})",
                        p2.latest_elixir == expected_cap, f"got {p2.latest_elixir}")
        else:
            time.sleep(5)
            print(f"  {CYAN}P1 final: elixir={p1.latest_elixir}, cap={p1.latest_elixir_cap}{NC}")
            print(f"  {CYAN}P2 final: elixir={p2.latest_elixir}, cap={p2.latest_elixir_cap}{NC}")
            assert_true("R5: P1 cap = 8", p1.latest_elixir_cap == 8, f"got {p1.latest_elixir_cap}")
            assert_true("R5: P2 cap = 8", p2.latest_elixir_cap == 8, f"got {p2.latest_elixir_cap}")

    # ── Cleanup ──
    p1.close()
    p2.close()

    print(f"\n{YELLOW}Phase history:{NC}")
    print(f"  P1: {p1.phase_history}")
    print(f"  P2: {p2.phase_history}")

    print(f"\n{'═' * 50}")
    print(f"Results: {GREEN}{passed} passed{NC}, {RED}{failed} failed{NC}, {total} total")
    print(f"{'═' * 50}")

    sys.exit(1 if failed > 0 else 0)


if __name__ == "__main__":
    main()

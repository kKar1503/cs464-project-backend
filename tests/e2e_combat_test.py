"""
Integration test: One player selects cards, places them, auto-attacks the other leader
until game over. Runs twice — once with P1 attacking, once with P2 attacking.

Verifies:
  - Deck → DrawPile → Hand → Board → back to Deck lifecycle
  - Card charge timer (10s), auto-attack, leader counterattack
  - Game over when leader HP ≤ 0, winner_id in broadcast

Usage: python tests/e2e_combat_test.py
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
verbose = False
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
        self.latest_tick = 0
        self.latest_phase = None
        self.latest_params = None
        self.phase_history = []
        self._recv_thread = None

    @property
    def draw_pile(self):
        p = self.latest_params
        return p.get("draw_pile", []) if p else []

    @property
    def hand(self):
        p = self.latest_params
        return p.get("hand", []) if p else []

    @property
    def deck_size(self):
        p = self.latest_params
        return p.get("deck_size", -1) if p else -1

    @property
    def your_board(self):
        p = self.latest_params
        return p.get("your_board", []) if p else []

    @property
    def your_hp(self):
        p = self.latest_params
        return p.get("your_hp", 0) if p else 0

    @property
    def enemy_hp(self):
        p = self.latest_params
        return p.get("enemy_hp", 0) if p else 0

    @property
    def elixir(self):
        p = self.latest_params
        return p.get("elixir", 0) if p else 0

    @property
    def combat_log(self):
        p = self.latest_params
        return p.get("combat_log", []) if p else []

    @property
    def winner_id(self):
        p = self.latest_params
        return p.get("winner_id", 0) if p else 0

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
                if verbose:
                    print(f"    {CYAN}[{self.name} recv]{NC} {json.dumps(msg)}")
                sv = msg.get("state_view")
                if isinstance(sv, dict):
                    tn = sv.get("tick_number", 0)
                    with lock:
                        if tn > self.latest_tick:
                            self.latest_tick = tn
                params = msg.get("params")
                if isinstance(params, dict) and params.get("phase"):
                    with lock:
                        self.latest_phase = params["phase"]
                        self.latest_params = params
                        if not self.phase_history or self.phase_history[-1] != params["phase"]:
                            self.phase_history.append(params["phase"])
            except websocket.WebSocketTimeoutException:
                continue
            except Exception:
                break

    def send_action(self, action, params=None):
        if not self.ws:
            return
        with lock:
            seq = self.latest_tick
        self.ws.send(json.dumps({
            "action": action,
            "params": params or {},
            "state_hash_after": 0,
            "sequence_number": seq,
        }))

    def wait_for_phase(self, phase, timeout=40):
        start = time.time()
        while time.time() - start < timeout:
            with lock:
                if self.latest_phase == phase:
                    return True
            time.sleep(0.25)
        return False

    def wait_for_condition(self, fn, timeout=60, poll=0.5):
        start = time.time()
        while time.time() - start < timeout:
            with lock:
                if fn():
                    return True
            time.sleep(poll)
        return False

    def close(self):
        self.connected = False
        if self.ws:
            try:
                self.ws.close()
            except Exception:
                pass


def matchmake(p1_token, p2_token):
    requests.post(f"{MATCHMAKING_URL}/matchmaking/queue",
                  headers={"Authorization": f"Bearer {p1_token}"})
    requests.post(f"{MATCHMAKING_URL}/matchmaking/queue",
                  headers={"Authorization": f"Bearer {p2_token}"})
    session_id = None
    for _ in range(30):
        time.sleep(0.5)
        r = requests.get(f"{MATCHMAKING_URL}/matchmaking/match",
                         headers={"Authorization": f"Bearer {p1_token}"})
        if r.json().get("matched"):
            session_id = r.json()["session_id"]
            break
    if not session_id:
        return None
    requests.get(f"{MATCHMAKING_URL}/matchmaking/match",
                 headers={"Authorization": f"Bearer {p2_token}"})
    requests.post(f"{MATCHMAKING_URL}/matchmaking/match/accept",
                  headers={"Authorization": f"Bearer {p1_token}"},
                  json={"session_id": session_id})
    requests.post(f"{MATCHMAKING_URL}/matchmaking/match/accept",
                  headers={"Authorization": f"Bearer {p2_token}"},
                  json={"session_id": session_id})
    return session_id


def run_combat_test(attacker_label, attacker_token, defender_token):
    """
    Run a full combat test where `attacker` places cards and `defender` does nothing.
    Verifies attacker wins (defender HP ≤ 0).
    """
    global passed, failed

    print(f"\n{'═' * 60}")
    print(f"{YELLOW}=== Combat Test: {attacker_label} attacks ==={NC}")
    print(f"{'═' * 60}\n")

    # ── Matchmake ──
    print(f"{YELLOW}Setup: Matchmake{NC}")
    session_id = matchmake(attacker_token, defender_token)
    assert_true("Match found", session_id is not None)
    if not session_id:
        return
    print(f"  Session: {session_id}")

    # ── Connect ──
    print(f"\n{YELLOW}Connect via WebSocket{NC}")
    atk = GamePlayer("ATK", attacker_token, session_id)
    dfn = GamePlayer("DFN", defender_token, session_id)
    atk.connect()
    dfn.connect()
    assert_true("Attacker connected", atk.connected, atk.error or "")
    assert_true("Defender connected", dfn.connected, dfn.error or "")
    if not atk.connected or not dfn.connected:
        return

    time.sleep(3)

    # ── Round 1 Pre-turn: select cards ──
    print(f"\n{YELLOW}Round 1 Pre-turn: Select cards{NC}")
    atk.wait_for_phase("PRE_TURN", timeout=15)
    dfn.wait_for_phase("PRE_TURN", timeout=5)
    time.sleep(2)

    dp = atk.draw_pile
    print(f"  {CYAN}ATK draw pile: {len(dp)} cards — {[c['card_name'] for c in dp]}{NC}")
    print(f"  {CYAN}ATK deck size: {atk.deck_size}{NC}")
    print(f"  {CYAN}DFN draw pile: {len(dfn.draw_pile)} cards{NC}")
    assert_true("ATK draw pile has cards", len(dp) > 0)

    initial_deck_size = atk.deck_size

    if len(dp) >= 2:
        card_to_play = dp[0]
        print(f"  ATK selecting card {dp[0]['card_id']} ({dp[0]['card_name']})")
        atk.send_action("SELECT_CARD", {"card_id": dp[0]["card_id"]})
        time.sleep(0.5)
        print(f"  ATK selecting card {dp[1]['card_id']} ({dp[1]['card_name']})")
        atk.send_action("SELECT_CARD", {"card_id": dp[1]["card_id"]})
        time.sleep(2)

        print(f"  {CYAN}ATK hand: {len(atk.hand)} cards — {[c['card_name'] for c in atk.hand]}{NC}")
        assert_true("ATK hand has 2 cards", len(atk.hand) == 2, f"got {len(atk.hand)}")
    else:
        print(f"  {RED}Not enough cards in draw pile{NC}")
        atk.close()
        dfn.close()
        return

    # ── Wait for ACTIVE ──
    print(f"\n{YELLOW}Wait for ACTIVE{NC}")
    atk.wait_for_phase("ACTIVE", timeout=15)
    dfn.wait_for_phase("ACTIVE", timeout=5)

    # ── Place first card ──
    card_id = card_to_play["card_id"]
    card_name = card_to_play["card_name"]
    card_atk = card_to_play["attack"]
    card_cost = card_to_play["mana_cost"]
    print(f"\n{YELLOW}Place '{card_name}' (atk={card_atk}, cost={card_cost}) at [0][0]{NC}")

    if atk.elixir < card_cost:
        atk.wait_for_condition(lambda: atk.elixir >= card_cost, timeout=30)

    pre_play_deck = atk.deck_size
    atk.send_action("CARD_PLACED", {"card_id": card_id, "row": 0, "col": 0})
    time.sleep(2)

    print(f"  {CYAN}ATK board: {[(c['card_name'], c['row'], c['col']) for c in atk.your_board]}{NC}")
    print(f"  {CYAN}ATK hand: {len(atk.hand)}, deck: {atk.deck_size}{NC}")
    assert_true("Card on board", any(c["card_id"] == card_id for c in atk.your_board))
    assert_true("Hand decreased", len(atk.hand) == 1, f"got {len(atk.hand)}")
    assert_true("Card returned to deck", atk.deck_size == pre_play_deck + 1)

    # ── Wait for first attack ──
    print(f"\n{YELLOW}Wait for first auto-attack (10s){NC}")
    print(f"  {CYAN}DFN HP before: {dfn.your_hp}{NC}")
    hp_dropped = atk.wait_for_condition(lambda: atk.enemy_hp < 250, timeout=15)
    assert_true("Defender took damage", hp_dropped, f"enemy HP: {atk.enemy_hp}")

    if hp_dropped:
        print(f"  {CYAN}ATK sees enemy HP: {atk.enemy_hp}{NC}")
        print(f"  {CYAN}DFN sees own HP: {dfn.your_hp}{NC}")
        expected = 250 - card_atk
        assert_true(f"DFN HP = {expected}", atk.enemy_hp == expected, f"got {atk.enemy_hp}")

        log = atk.combat_log
        if log:
            print(f"  {CYAN}Combat log: {json.dumps(log, indent=2)}{NC}")
            assert_true("First event is attack", log[0].get("type") == "attack",
                        f"type: {log[0].get('type')}")
            assert_true("Attack targets leader", log[0].get("target_is_leader"),
                        f"log: {log[0]}")
            if len(log) >= 2:
                assert_true("Second event is counter_attack", log[1].get("type") == "counter_attack",
                            f"type: {log[1].get('type')}")

    # ── Continuous combat until game over ──
    print(f"\n{YELLOW}Continuous combat until game over{NC}")
    max_wait = 600
    start = time.time()
    last_phase = None

    while time.time() - start < max_wait:
        time.sleep(2)
        with lock:
            phase = atk.latest_phase
            enemy_hp = atk.enemy_hp
            your_hp = atk.your_hp
            board = atk.your_board
            hand = atk.hand
            draw_pile = atk.draw_pile
            elixir = atk.elixir
            winner = atk.winner_id

        if phase == "GAME_OVER":
            print(f"\n{YELLOW}Game Over!{NC}")
            print(f"  {CYAN}ATK HP: {your_hp}, DFN HP: {enemy_hp}{NC}")
            print(f"  {CYAN}ATK winner_id: {winner}{NC}")
            print(f"  {CYAN}DFN winner_id: {dfn.winner_id}{NC}")
            assert_true("Game ended", True)
            assert_true("Defender HP <= 0", enemy_hp <= 0, f"DFN HP: {enemy_hp}")
            assert_true("Winner ID is set", winner > 0, f"got {winner}")
            assert_true("Both see same winner", atk.winner_id == dfn.winner_id,
                        f"ATK={atk.winner_id}, DFN={dfn.winner_id}")
            break

        if phase != last_phase:
            print(f"  {CYAN}[{int(time.time()-start)}s] → {phase}, DFN HP={enemy_hp}, board={len(board)}, hand={len(hand)}, draw={len(draw_pile)}, elixir={elixir}{NC}")
            last_phase = phase

        # PRE_TURN: select cards one at a time
        if phase == "PRE_TURN" and draw_pile:
            select_count = min(4 - len(hand), len(draw_pile))
            for card in draw_pile[:select_count]:
                atk.send_action("SELECT_CARD", {"card_id": card["card_id"]})
                time.sleep(0.3)

        # ACTIVE: place cards
        if phase == "ACTIVE" and hand and len(board) < 6:
            for card in hand:
                if elixir >= card["mana_cost"]:
                    occupied = {(c["row"], c["col"]) for c in board}
                    for r in range(2):
                        for c in range(3):
                            if (r, c) not in occupied:
                                atk.send_action("CARD_PLACED", {"card_id": card["card_id"], "row": r, "col": c})
                                time.sleep(1)
                                break
                        else:
                            continue
                        break
                    break

    else:
        assert_true("Game should have ended", False, f"phase={atk.latest_phase}, DFN HP={atk.enemy_hp}")

    print(f"\n  Phase history ATK: {atk.phase_history}")
    print(f"  Phase history DFN: {dfn.phase_history}")

    atk.close()
    dfn.close()


def main():
    global passed, failed

    import argparse
    parser = argparse.ArgumentParser(description="E2E Combat Test")
    parser.add_argument("--attacker", choices=["p1", "p2"], default="p1",
                        help="Which player attacks: p1 (default) or p2")
    parser.add_argument("--verbose", "-v", action="store_true",
                        help="Print raw WebSocket messages")
    args = parser.parse_args()

    global verbose
    verbose = args.verbose

    ts = int(time.time())
    password = "testpassword123"

    print(f"{YELLOW}Registering players...{NC}")
    p1_token, _ = register_and_login(f"cbt_p1_{ts}", password)
    p2_token, _ = register_and_login(f"cbt_p2_{ts}", password)

    if args.attacker == "p1":
        run_combat_test("P1 attacks P2", p1_token, p2_token)
    else:
        run_combat_test("P2 attacks P1", p2_token, p1_token)

    print(f"\n{'═' * 60}")
    print(f"Results: {GREEN}{passed} passed{NC}, {RED}{failed} failed{NC}, {total} total")
    print(f"{'═' * 60}")

    sys.exit(1 if failed > 0 else 0)


if __name__ == "__main__":
    main()

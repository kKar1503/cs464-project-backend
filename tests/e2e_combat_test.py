"""
Integration test: P1 selects cards, places one on board, card auto-attacks P2 leader
repeatedly until P2 leader dies. Verifies the full card lifecycle:
  Deck → DrawPile → Hand → Board → back to Deck

Also verifies:
  - Card charge timer (10s)
  - Auto-attack dealing damage to leader
  - Leader counterattack
  - Game over when leader HP ≤ 0

Prerequisites:
    - Auth, Matchmaking, Deck, Gameplay services running
    - DB migrated and seeded

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
    def attack_log(self):
        p = self.latest_params
        return p.get("attack_log", []) if p else []

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
    """Queue both players, wait for match, accept."""
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


def main():
    global passed, failed

    ts = int(time.time())
    password = "testpassword123"

    print(f"{YELLOW}=== E2E Combat Test ==={NC}\n")

    # ── Setup ──
    print(f"{YELLOW}Step 1: Register, login, matchmake{NC}")
    p1_token, p1_id = register_and_login(f"combat_p1_{ts}", password)
    p2_token, p2_id = register_and_login(f"combat_p2_{ts}", password)

    session_id = matchmake(p1_token, p2_token)
    assert_true("Match found", session_id is not None)
    if not session_id:
        sys.exit(1)
    print(f"  Session: {session_id}")

    # ── Connect ──
    print(f"\n{YELLOW}Step 2: Connect via WebSocket{NC}")
    p1 = GamePlayer("P1", p1_token, session_id)
    p2 = GamePlayer("P2", p2_token, session_id)
    p1.connect()
    p2.connect()
    assert_true("P1 connected", p1.connected, p1.error or "")
    assert_true("P2 connected", p2.connected, p2.error or "")
    if not p1.connected or not p2.connected:
        sys.exit(1)

    time.sleep(3)

    # ── Round 1: Pre-turn ──
    print(f"\n{YELLOW}Step 3: Round 1 — Pre-turn{NC}")
    got_pt = p1.wait_for_phase("PRE_TURN", timeout=15)
    p2.wait_for_phase("PRE_TURN", timeout=5)
    assert_true("P1 sees PRE_TURN", got_pt)

    time.sleep(2)  # let tick updates arrive with draw pile data

    # ── Verify draw pile ──
    dp = p1.draw_pile
    print(f"  {CYAN}P1 draw pile: {len(dp)} cards — {[c['card_name'] for c in dp]}{NC}")
    print(f"  {CYAN}P1 hand: {len(p1.hand)} cards{NC}")
    print(f"  {CYAN}P1 deck size: {p1.deck_size}{NC}")
    assert_true("P1 draw pile has cards", len(dp) > 0, f"draw pile is empty")

    initial_deck_size = p1.deck_size
    initial_dp_size = len(dp)

    # ── Select 2 cards from draw pile to hand ──
    if len(dp) >= 2:
        card_ids_to_select = [dp[0]["card_id"], dp[1]["card_id"]]
        card_to_play = dp[0]  # we'll play this one
        print(f"\n{YELLOW}Step 4: Select 2 cards from draw pile{NC}")
        print(f"  Selecting: {card_ids_to_select}")
        p1.send_action("SELECT_CARDS", {"card_ids": card_ids_to_select})
        p2.send_action("SELECT_CARDS", {"card_ids": []})  # P2 skips
        time.sleep(2)

        print(f"  {CYAN}P1 draw pile after select: {len(p1.draw_pile)} cards{NC}")
        print(f"  {CYAN}P1 hand after select: {len(p1.hand)} cards — {[c['card_name'] for c in p1.hand]}{NC}")
        print(f"  {CYAN}P1 deck size: {p1.deck_size}{NC}")

        assert_true("P1 hand has 2 cards", len(p1.hand) == 2, f"got {len(p1.hand)}")
        assert_true("P1 draw pile decreased by 2", len(p1.draw_pile) == initial_dp_size - 2,
                     f"got {len(p1.draw_pile)}, expected {initial_dp_size - 2}")
    else:
        print(f"  {RED}Not enough cards in draw pile, aborting{NC}")
        p1.close()
        p2.close()
        sys.exit(1)

    # ── Wait for ACTIVE ──
    print(f"\n{YELLOW}Step 5: Wait for ACTIVE phase{NC}")
    got_active = p1.wait_for_phase("ACTIVE", timeout=15)
    p2.wait_for_phase("ACTIVE", timeout=5)
    assert_true("P1 sees ACTIVE", got_active)

    time.sleep(1)
    pre_play_deck_size = p1.deck_size
    print(f"  {CYAN}P1 elixir: {p1.elixir}{NC}")
    print(f"  {CYAN}P1 hand: {[c['card_name'] for c in p1.hand]}{NC}")

    # ── Place 1 card on board ──
    card_id = card_to_play["card_id"]
    card_name = card_to_play["card_name"]
    card_atk = card_to_play["attack"]
    card_cost = card_to_play["mana_cost"]

    print(f"\n{YELLOW}Step 6: Place card '{card_name}' (id={card_id}, atk={card_atk}, cost={card_cost}) at [0][0]{NC}")

    # Wait for enough elixir
    if p1.elixir < card_cost:
        print(f"  Waiting for elixir ({p1.elixir}/{card_cost})...")
        p1.wait_for_condition(lambda: p1.elixir >= card_cost, timeout=30)

    p1.send_action("CARD_PLACED", {"card_id": card_id, "row": 0, "col": 0})
    time.sleep(2)

    # Verify card is on board
    board = p1.your_board
    print(f"  {CYAN}P1 board: {len(board)} cards — {[(c['card_name'], c['row'], c['col']) for c in board]}{NC}")
    print(f"  {CYAN}P1 hand after play: {len(p1.hand)} cards{NC}")
    print(f"  {CYAN}P1 deck size after play: {p1.deck_size}{NC}")

    assert_true("Card is on board", len(board) >= 1 and any(c["card_id"] == card_id for c in board),
                f"board: {board}")
    assert_true("Hand decreased by 1", len(p1.hand) == 1, f"got {len(p1.hand)}")
    assert_true("Card returned to deck (deck size +1)", p1.deck_size == pre_play_deck_size + 1,
                f"deck was {pre_play_deck_size}, now {p1.deck_size}")

    # ── Wait for auto-attack ──
    print(f"\n{YELLOW}Step 7: Wait for auto-attack (10s charge){NC}")
    initial_enemy_hp = p2.your_hp  # from P2's perspective, their own HP
    print(f"  {CYAN}P2 HP before attack: {initial_enemy_hp}{NC}")
    print(f"  {CYAN}P1 sees enemy HP: {p1.enemy_hp}{NC}")

    # Wait for P2 HP to drop (attack happens after 10s charge)
    hp_dropped = p1.wait_for_condition(lambda: p1.enemy_hp < 250, timeout=15)
    assert_true("P2 leader took damage", hp_dropped, f"enemy HP: {p1.enemy_hp}")

    if hp_dropped:
        print(f"  {CYAN}P1 sees enemy HP after first attack: {p1.enemy_hp}{NC}")
        print(f"  {CYAN}P2 sees own HP: {p2.your_hp}{NC}")
        expected_hp = 250 - card_atk
        assert_true(f"P2 HP = {expected_hp} (250 - {card_atk})", p1.enemy_hp == expected_hp,
                    f"got {p1.enemy_hp}")

        # Check attack log
        log = p1.attack_log
        if log:
            print(f"  {CYAN}Attack log: {json.dumps(log, indent=2)}{NC}")
            assert_true("Attack targets leader", log[0].get("target_is_leader"),
                        f"log: {log[0]}")
            assert_true("Counter damage from leader", log[0].get("counter_damage", 0) > 0,
                        f"counter: {log[0].get('counter_damage')}")

    # ── Continuously place cards and attack ──
    # Wind Walker has 5 HP, leader counterattacks for 10, so it dies after 1 attack.
    # We need to keep placing cards each round. Each attack does card_atk damage.
    # With 5 damage per attack and P2 at 245 HP, we need many rounds.
    # Strategy: each round, select cards from draw pile and place one.
    print(f"\n{YELLOW}Step 8: Continuous combat — place cards each round{NC}")

    max_wait = 600  # 10 minutes max
    start = time.time()
    last_phase = None

    while time.time() - start < max_wait:
        time.sleep(2)
        with lock:
            phase = p1.latest_phase
            enemy_hp = p1.enemy_hp
            your_hp = p1.your_hp
            board_cards = p1.your_board
            hand = p1.hand
            draw_pile = p1.draw_pile
            elixir = p1.elixir

        if phase == "GAME_OVER":
            print(f"\n{YELLOW}Step 9: Game Over!{NC}")
            assert_true("Game ended", True)
            assert_true("P2 HP <= 0", enemy_hp <= 0, f"P2 HP: {enemy_hp}")
            print(f"  {CYAN}P1 HP: {your_hp}{NC}")
            print(f"  {CYAN}P2 HP: {enemy_hp}{NC}")
            break

        # On phase change, log
        if phase != last_phase:
            print(f"  {CYAN}[{int(time.time()-start)}s] → {phase}, P2 HP={enemy_hp}, P1 board={len(board_cards)}, hand={len(hand)}, draw={len(draw_pile)}, elixir={elixir}{NC}")
            last_phase = phase

        # During PRE_TURN: select up to 4 cards from draw pile
        if phase == "PRE_TURN" and draw_pile:
            select_count = min(4 - len(hand), len(draw_pile))
            if select_count > 0:
                ids_to_select = [c["card_id"] for c in draw_pile[:select_count]]
                p1.send_action("SELECT_CARDS", {"card_ids": ids_to_select})
            p2.send_action("SELECT_CARDS", {"card_ids": []})

        # During ACTIVE: place a card if we have one and enough elixir and board space
        if phase == "ACTIVE" and hand and len(board_cards) < 6:
            for card in hand:
                if elixir >= card["mana_cost"]:
                    # Find empty slot
                    occupied = {(c["row"], c["col"]) for c in board_cards}
                    for r in range(2):
                        for c in range(3):
                            if (r, c) not in occupied:
                                p1.send_action("CARD_PLACED", {
                                    "card_id": card["card_id"], "row": r, "col": c,
                                })
                                print(f"  {CYAN}[{int(time.time()-start)}s] Placed '{card['card_name']}' at [{r}][{c}], P2 HP={enemy_hp}{NC}")
                                time.sleep(1)  # let it process
                                break
                        else:
                            continue
                        break
                    break

    else:
        assert_true("Game should have ended within 10 minutes", False,
                    f"phase={p1.latest_phase}, P2 HP={p1.enemy_hp}")

    # ── Summary ──
    print(f"\n{YELLOW}Phase history:{NC}")
    print(f"  P1: {p1.phase_history}")
    print(f"  P2: {p2.phase_history}")

    p1.close()
    p2.close()

    print(f"\n{'═' * 55}")
    print(f"Results: {GREEN}{passed} passed{NC}, {RED}{failed} failed{NC}, {total} total")
    print(f"{'═' * 55}")

    sys.exit(1 if failed > 0 else 0)


if __name__ == "__main__":
    main()

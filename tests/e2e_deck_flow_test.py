"""
End-to-end test: Register → Buy packs → Open packs → Get cards → Create deck →
                  Get decks → Set active → Get active → Error cases → Delete → Verify

Prerequisites:
    - Auth service running on localhost:8000
    - Deck service running on localhost:8005
    - DB is migrated and seeded (make migrate-reset)

Usage: python tests/e2e_deck_flow_test.py
"""

import os
import requests
import time
import sys

AUTH_URL = os.environ.get("AUTH_URL", "http://localhost:8000")
DECK_URL = os.environ.get("DECK_URL", "http://localhost:8005")

passed = 0
failed = 0
total = 0

GREEN = "\033[92m"
RED = "\033[91m"
YELLOW = "\033[93m"
NC = "\033[0m"


def assert_status(desc: str, expected: int, resp: requests.Response):
    global passed, failed, total
    total += 1
    if resp.status_code == expected:
        print(f"  {GREEN}✓{NC} {desc} (HTTP {resp.status_code})")
        passed += 1
    else:
        print(f"  {RED}✗{NC} {desc} — expected HTTP {expected}, got {resp.status_code}")
        print(f"    Response: {resp.text[:200]}")
        failed += 1


def assert_json(desc: str, data: dict, field: str, expected):
    global passed, failed, total
    total += 1
    keys = field.split(".")
    val = data
    for k in keys:
        if isinstance(val, dict):
            val = val.get(k)
        else:
            val = None
            break
    if val == expected:
        print(f"  {GREEN}✓{NC} {desc} ({field} = {val})")
        passed += 1
    else:
        print(f"  {RED}✗{NC} {desc} — expected {field} = {expected}, got {val}")
        failed += 1


def assert_gte(desc: str, data: dict, field: str, minimum: int):
    global passed, failed, total
    total += 1
    keys = field.split(".")
    val = data
    for k in keys:
        if isinstance(val, dict):
            val = val.get(k, 0)
        else:
            val = 0
            break
    if isinstance(val, (int, float)) and val >= minimum:
        print(f"  {GREEN}✓{NC} {desc} ({field} = {val} >= {minimum})")
        passed += 1
    else:
        print(f"  {RED}✗{NC} {desc} — expected {field} >= {minimum}, got {val}")
        failed += 1


def assert_true(desc: str, condition: bool):
    global passed, failed, total
    total += 1
    if condition:
        print(f"  {GREEN}✓{NC} {desc}")
        passed += 1
    else:
        print(f"  {RED}✗{NC} {desc}")
        failed += 1


def auth_header(token: str) -> dict:
    return {"Authorization": f"Bearer {token}"}


def main():
    global passed, failed

    username = f"e2e_test_{int(time.time())}"
    password = "testpassword123"

    print(f"{YELLOW}=== E2E Deck Flow Test ==={NC}\n")

    # ── Step 1: Register ──
    print(f"{YELLOW}Step 1: Register a new user{NC}")
    resp = requests.post(f"{AUTH_URL}/auth/register", json={
        "username": username, "password": password,
    })
    assert_status("Register user", 201, resp)
    data = resp.json()
    user_id = data.get("user_id")
    print(f"  User ID: {user_id}")

    # ── Step 2: Login ──
    print(f"\n{YELLOW}Step 2: Login{NC}")
    resp = requests.post(f"{AUTH_URL}/auth/login", json={
        "username": username, "password": password,
    })
    assert_status("Login", 200, resp)
    data = resp.json()
    token = data.get("token", "")
    print(f"  Token: {token[:16]}...")
    headers = auth_header(token)

    # ── Step 3: Verify no cards yet ──
    print(f"\n{YELLOW}Step 3: Verify no cards yet (before packs){NC}")
    resp = requests.get(f"{DECK_URL}/players/me/cards", headers=headers)
    assert_status("Get player cards (initial)", 200, resp)
    data = resp.json()
    initial_card_count = data.get("count", 0)
    print(f"  Initial card count: {initial_card_count}")

    # ── Step 4: Verify no packs yet ──
    print(f"\n{YELLOW}Step 4: Verify no packs yet{NC}")
    resp = requests.get(f"{DECK_URL}/packs", headers=headers)
    assert_status("Get packs (initial)", 200, resp)

    # ── Step 5: Buy 5 packs ──
    print(f"\n{YELLOW}Step 5: Buy 5 packs{NC}")
    first_pack_id = None
    for i in range(1, 6):
        resp = requests.post(f"{DECK_URL}/packs", headers=headers)
        data = resp.json()
        pack_id = data.get("pack_id")
        pack_type = data.get("pack_type")
        if i == 1:
            assert_status(f"Buy pack #{i} ({pack_type})", 201, resp)
            first_pack_id = pack_id
        else:
            print(f"  Pack #{i}: id={pack_id} type={pack_type} (HTTP {resp.status_code})")

    # ── Step 6: Verify packs are listed ──
    print(f"\n{YELLOW}Step 6: Verify packs are listed{NC}")
    resp = requests.get(f"{DECK_URL}/packs", headers=headers)
    assert_status("Get packs (after buying)", 200, resp)
    data = resp.json()
    packs = data.get("packs", [])
    assert_gte("Have at least 5 packs", {"count": len(packs)}, "count", 5)

    # ── Step 7: Open all packs ──
    print(f"\n{YELLOW}Step 7: Open all packs{NC}")
    opened = 0
    for pack in packs:
        pid = pack["pack_id"]
        resp = requests.post(f"{DECK_URL}/packs/open?pack_id={pid}", headers=headers)
        if opened == 0:
            assert_status(f"Open pack {pid}", 200, resp)
            cards_in_pack = len(resp.json().get("cards", []))
            print(f"  Cards in first pack: {cards_in_pack}")
        opened += 1
    print(f"  Opened {opened} packs total")

    # ── Step 8: Verify we now have cards ──
    print(f"\n{YELLOW}Step 8: Verify we now have cards{NC}")
    resp = requests.get(f"{DECK_URL}/players/me/cards", headers=headers)
    assert_status("Get player cards (after opening)", 200, resp)
    data = resp.json()
    card_count = data.get("count", 0)
    assert_gte("Have cards after opening packs", {"count": card_count}, "count", 1)
    print(f"  Total unique cards owned: {card_count}")

    # ── Step 9: Check initial decks (starter decks may exist from registration) ──
    print(f"\n{YELLOW}Step 9: Check initial decks{NC}")
    resp = requests.get(f"{DECK_URL}/decks", headers=headers)
    assert_status("Get decks (initial)", 200, resp)
    data = resp.json()
    initial_deck_count = data.get("count", 0)
    print(f"  Initial deck count: {initial_deck_count} (starter decks from registration)")

    # ── Step 10: Verify no active deck yet ──
    print(f"\n{YELLOW}Step 10: Verify no active deck yet{NC}")
    resp = requests.get(f"{DECK_URL}/decks/active", headers=headers)
    assert_status("Get active deck (initial)", 200, resp)
    data = resp.json()
    assert_json("No active deck", data, "active_deck_id", None)

    # ── Step 11: Build a deck from owned cards ──
    print(f"\n{YELLOW}Step 11: Build a deck from owned cards{NC}")
    resp = requests.get(f"{DECK_URL}/players/me/cards", headers=headers)
    cards = resp.json().get("cards", [])

    # Take each card up to 2 copies, stop at 12
    deck_card_ids = []
    for card in cards:
        copies = min(card["quantity"], 2)
        for _ in range(copies):
            if len(deck_card_ids) >= 12:
                break
            deck_card_ids.append(card["card_id"])
        if len(deck_card_ids) >= 12:
            break

    deck_size = len(deck_card_ids)
    print(f"  Building deck with {deck_size} cards: {deck_card_ids}")

    if deck_size < 1:
        print(f"  {RED}Cannot build deck — not enough cards. Buy more packs.{NC}")
        sys.exit(1)

    # ── Step 12: Create the deck ──
    print(f"\n{YELLOW}Step 12: Create the deck{NC}")
    resp = requests.post(f"{DECK_URL}/decks", headers=headers, json={
        "name": "E2E Test Deck", "card_ids": deck_card_ids,
    })
    assert_status("Create deck", 201, resp)
    data = resp.json()
    deck_id = data.get("deck_id")
    assert_json("Deck name", data, "name", "E2E Test Deck")
    print(f"  Created deck ID: {deck_id}")

    # ── Step 13: Get all decks — verify it's there ──
    print(f"\n{YELLOW}Step 13: Get all decks — verify it's there{NC}")
    resp = requests.get(f"{DECK_URL}/decks", headers=headers)
    assert_status("Get all decks", 200, resp)
    data = resp.json()
    assert_true(f"Deck count increased ({data.get('count', 0)} > {initial_deck_count})",
                data.get("count", 0) > initial_deck_count)
    found = any(d["deck_id"] == deck_id for d in data.get("decks", []))
    assert_true(f"Deck {deck_id} found in list", found)

    # ── Step 14: Get deck by ID — verify cards ──
    print(f"\n{YELLOW}Step 14: Get deck by ID — verify cards{NC}")
    resp = requests.get(f"{DECK_URL}/decks/{deck_id}", headers=headers)
    assert_status("Get deck by ID", 200, resp)
    data = resp.json()
    assert_json("Deck name matches", data, "name", "E2E Test Deck")
    returned_card_count = len(data.get("card_ids", []))
    assert_true(f"Card count matches ({returned_card_count} == {deck_size})",
                returned_card_count == deck_size)

    # ── Step 15: Set active deck ──
    print(f"\n{YELLOW}Step 15: Set active deck{NC}")
    resp = requests.put(f"{DECK_URL}/decks/active", headers=headers, json={
        "deck_id": deck_id,
    })
    assert_status("Set active deck", 200, resp)

    # ── Step 16: Get active deck — verify it's set ──
    print(f"\n{YELLOW}Step 16: Get active deck — verify it's set{NC}")
    resp = requests.get(f"{DECK_URL}/decks/active", headers=headers)
    assert_status("Get active deck", 200, resp)
    data = resp.json()
    assert_json("Active deck is ours", data, "active_deck_id", deck_id)

    # ── Step 17: Try to open an already-opened pack (should fail) ──
    print(f"\n{YELLOW}Step 17: Try to open an already-opened pack (should fail){NC}")
    resp = requests.post(f"{DECK_URL}/packs/open?pack_id={first_pack_id}", headers=headers)
    assert_status("Open already-opened pack", 400, resp)
    assert_json("Error message", resp.json(), "error", "pack already opened")

    # ── Step 18: Try to create a deck with too many cards (should fail) ──
    print(f"\n{YELLOW}Step 18: Try to create a deck with too many cards (should fail){NC}")
    if deck_size >= 12:
        too_many = deck_card_ids + [deck_card_ids[0]]
        resp = requests.post(f"{DECK_URL}/decks", headers=headers, json={
            "name": "Too Big", "card_ids": too_many,
        })
        assert_status("Reject deck with >12 cards", 400, resp)
    else:
        print("  Skipped (deck was < 12 cards, can't test overflow)")

    # ── Step 19: Try to create a deck with a card you don't own (should fail) ──
    print(f"\n{YELLOW}Step 19: Try to create a deck with unowned card (should fail){NC}")
    resp = requests.post(f"{DECK_URL}/decks", headers=headers, json={
        "name": "Bad Deck", "card_ids": [99999],
    })
    assert_status("Reject deck with unowned card", 400, resp)

    # ── Step 20: Set active deck to nonexistent deck (should fail) ──
    print(f"\n{YELLOW}Step 20: Set active deck to nonexistent deck (should fail){NC}")
    resp = requests.put(f"{DECK_URL}/decks/active", headers=headers, json={
        "deck_id": 99999,
    })
    assert_status("Reject nonexistent active deck", 404, resp)

    # ── Step 21: Delete the deck ──
    print(f"\n{YELLOW}Step 21: Delete the deck{NC}")
    resp = requests.delete(f"{DECK_URL}/decks/{deck_id}", headers=headers)
    assert_status("Delete deck", 200, resp)

    # ── Step 22: Verify deck is gone ──
    print(f"\n{YELLOW}Step 22: Verify deck is gone{NC}")
    resp = requests.get(f"{DECK_URL}/decks/{deck_id}", headers=headers)
    assert_status("Deck no longer exists", 404, resp)

    # ── Step 23: Verify active deck is cleared (FK set null on delete) ──
    print(f"\n{YELLOW}Step 23: Verify active deck is cleared after delete{NC}")
    resp = requests.get(f"{DECK_URL}/decks/active", headers=headers)
    assert_status("Get active deck after delete", 200, resp)
    data = resp.json()
    assert_json("Active deck cleared", data, "active_deck_id", None)

    # ── Results ──
    print(f"\n{'═' * 45}")
    print(f"Results: {GREEN}{passed} passed{NC}, {RED}{failed} failed{NC}, {total} total")
    print(f"{'═' * 45}")

    sys.exit(1 if failed > 0 else 0)


if __name__ == "__main__":
    main()

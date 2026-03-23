#!/usr/bin/env python3
"""
Integration test for matchmaking service.
Tests the full flow: register users -> join queue -> verify match.
"""

import requests
import time
import sys
import random
import string

AUTH_SERVICE_URL = "http://localhost:8000"
MATCHMAKING_SERVICE_URL = "http://localhost:8001"


def generate_random_username():
    """Generate a random username to avoid conflicts."""
    return f"testuser_{''.join(random.choices(string.ascii_lowercase + string.digits, k=8))}"


def register_user(username, password):
    """Register a new user via auth service."""
    url = f"{AUTH_SERVICE_URL}/auth/register"
    payload = {
        "username": username,
        "password": password
    }

    print(f"📝 Registering user: {username}")
    response = requests.post(url, json=payload)

    if response.status_code not in [200, 201]:
        print(f"❌ Failed to register {username}: {response.status_code}")
        print(f"   Response: {response.text}")
        sys.exit(1)

    data = response.json()
    print(f"✅ Registered {username} with user_id: {data['user_id']}")
    return data


def login_user(username, password):
    """Login a user and get auth token."""
    url = f"{AUTH_SERVICE_URL}/auth/login"
    payload = {
        "username": username,
        "password": password
    }

    print(f"🔐 Logging in user: {username}")
    response = requests.post(url, json=payload)

    if response.status_code != 200:
        print(f"❌ Failed to login {username}: {response.status_code}")
        print(f"   Response: {response.text}")
        sys.exit(1)

    data = response.json()
    print(f"✅ Logged in {username}, token: {data['token'][:16]}...")
    return data


def join_queue(token, username):
    """Add a user to the matchmaking queue."""
    url = f"{MATCHMAKING_SERVICE_URL}/matchmaking/join"
    headers = {"Authorization": f"Bearer {token}"}

    print(f"🎮 {username} joining queue...")
    response = requests.post(url, json={}, headers=headers)

    if response.status_code != 200:
        print(f"❌ Failed to join queue for {username}: {response.status_code}")
        print(f"   Response: {response.text}")
        sys.exit(1)

    data = response.json()
    print(f"✅ {username} joined queue (MMR: {data.get('mmr', 'N/A')})")
    return data


def check_match(token, username):
    """Check if a user has been matched."""
    url = f"{MATCHMAKING_SERVICE_URL}/matchmaking/match"
    headers = {"Authorization": f"Bearer {token}"}

    response = requests.get(url, headers=headers)

    if response.status_code != 200:
        print(f"❌ Failed to check match for {username}: {response.status_code}")
        print(f"   Response: {response.text}")
        sys.exit(1)

    return response.json()


def accept_match(session_id, token, username):
    """Accept a match."""
    url = f"{MATCHMAKING_SERVICE_URL}/matchmaking/accept"
    headers = {"Authorization": f"Bearer {token}"}
    payload = {"session_id": session_id}

    print(f"✅ {username} accepting match {session_id[:8]}...")
    response = requests.post(url, json=payload, headers=headers)

    if response.status_code != 200:
        print(f"❌ Failed to accept match for {username}: {response.status_code}")
        print(f"   Response: {response.text}")
        sys.exit(1)

    data = response.json()
    print(f"   Response: {data.get('message', 'N/A')}")
    return data


def reject_match(session_id, token, username):
    """Reject a match."""
    url = f"{MATCHMAKING_SERVICE_URL}/matchmaking/reject"
    headers = {"Authorization": f"Bearer {token}"}
    payload = {"session_id": session_id}

    print(f"❌ {username} rejecting match {session_id[:8]}...")
    response = requests.post(url, json=payload, headers=headers)

    if response.status_code != 200:
        print(f"❌ Failed to reject match for {username}: {response.status_code}")
        print(f"   Response: {response.text}")
        sys.exit(1)

    data = response.json()
    print(f"   Response: {data.get('message', 'N/A')}")
    return data


def main():
    print("=" * 60)
    print("🎯 Matchmaking Integration Test")
    print("=" * 60)
    print()

    # Step 1: Register two users
    print("Step 1: Registering users...")
    username1 = generate_random_username()
    username2 = generate_random_username()
    password = "password123"

    user1 = register_user(username1, password)
    user2 = register_user(username2, password)

    user1_id = user1["user_id"]
    user2_id = user2["user_id"]

    print()

    # Step 2: Login users and get tokens
    print("Step 2: Logging in users...")
    auth1 = login_user(username1, password)
    auth2 = login_user(username2, password)

    token1 = auth1["token"]
    token2 = auth2["token"]

    print()

    # Step 3: Join matchmaking queue
    print("Step 3: Joining matchmaking queue...")
    join_queue(token1, username1)
    join_queue(token2, username2)

    print()

    # Step 4: Wait for matchmaker to run (runs every 2 seconds)
    print("Step 4: Waiting for matchmaker to process queue...")
    wait_time = 4  # Wait 4 seconds to ensure at least one tick
    for i in range(wait_time, 0, -1):
        print(f"   ⏳ Waiting {i} seconds...")
        time.sleep(1)

    print()

    # Step 5: Check if both users were matched
    print("Step 5: Checking match results...")
    match1 = check_match(token1, username1)
    match2 = check_match(token2, username2)

    print()

    # Step 6: Verify results
    print("Step 6: Verifying match...")
    print(f"   User {user1_id} matched: {match1.get('matched', False)}")
    print(f"   User {user2_id} matched: {match2.get('matched', False)}")

    if not match1.get("matched"):
        print(f"❌ FAILED: User {user1_id} ({username1}) was not matched")
        sys.exit(1)

    if not match2.get("matched"):
        print(f"❌ FAILED: User {user2_id} ({username2}) was not matched")
        sys.exit(1)

    session1 = match1.get("session_id")
    session2 = match2.get("session_id")

    if session1 != session2:
        print(f"❌ FAILED: Users matched to different sessions")
        print(f"   User {user1_id} session: {session1}")
        print(f"   User {user2_id} session: {session2}")
        sys.exit(1)

    opponent1 = match1.get("opponent")
    opponent2 = match2.get("opponent")

    if opponent1 != username2:
        print(f"❌ FAILED: User {user1_id} opponent should be {username2}, got {opponent1}")
        sys.exit(1)

    if opponent2 != username1:
        print(f"❌ FAILED: User {user2_id} opponent should be {username1}, got {opponent2}")
        sys.exit(1)

    print()
    print("=" * 60)
    print("✅ SUCCESS! Both users were matched together")
    print("=" * 60)
    print(f"📊 Match Details:")
    print(f"   Session ID: {session1}")
    print(f"   Player 1: {username1} (MMR: {match1.get('your_mmr', 'N/A')})")
    print(f"   Player 2: {username2} (MMR: {match2.get('your_mmr', 'N/A')})")
    print("=" * 60)
    print()

    # Step 7: Both players accept the match
    print("Step 7: Testing match acceptance flow...")

    # Player 1 accepts first
    accept_result1 = accept_match(session1, token1, username1)
    if accept_result1.get("status") != "waiting":
        print(f"❌ FAILED: Expected status 'waiting' after first accept, got {accept_result1.get('status')}")
        sys.exit(1)

    if not accept_result1.get("player1_ready"):
        print(f"❌ FAILED: Player 1 should be ready after accepting")
        sys.exit(1)

    if accept_result1.get("player2_ready"):
        print(f"❌ FAILED: Player 2 should not be ready yet")
        sys.exit(1)

    print(f"   ✅ Player 1 ready, waiting for Player 2")

    # Small delay to simulate real-world timing
    time.sleep(1)

    # Player 2 accepts second
    accept_result2 = accept_match(session1, token2, username2)
    if accept_result2.get("status") != "in_progress":
        print(f"❌ FAILED: Expected status 'in_progress' after both accept, got {accept_result2.get('status')}")
        sys.exit(1)

    if not accept_result2.get("player1_ready") or not accept_result2.get("player2_ready"):
        print(f"❌ FAILED: Both players should be ready after accepting")
        sys.exit(1)

    print(f"   ✅ Both players ready, game started!")

    print()
    print("=" * 60)
    print("✅ FULL TEST PASSED!")
    print("=" * 60)
    print("   • Users registered successfully")
    print("   • Both joined matchmaking queue")
    print("   • Match found within expected time")
    print("   • Player 1 accepted (status: waiting)")
    print("   • Player 2 accepted (status: in_progress)")
    print("   • Game session started successfully")
    print("=" * 60)


if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        print("\n\n⚠️  Test interrupted by user")
        sys.exit(1)
    except Exception as e:
        print(f"\n❌ Unexpected error: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)

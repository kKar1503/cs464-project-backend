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


def join_queue(user_id):
    """Add a user to the matchmaking queue."""
    url = f"{MATCHMAKING_SERVICE_URL}/matchmaking/join"
    payload = {
        "user_id": user_id
    }

    print(f"🎮 User {user_id} joining queue...")
    response = requests.post(url, json=payload)

    if response.status_code != 200:
        print(f"❌ Failed to join queue for user {user_id}: {response.status_code}")
        print(f"   Response: {response.text}")
        sys.exit(1)

    data = response.json()
    print(f"✅ User {user_id} joined queue (MMR: {data.get('mmr', 'N/A')})")
    return data


def check_match(user_id):
    """Check if a user has been matched."""
    url = f"{MATCHMAKING_SERVICE_URL}/matchmaking/match"
    params = {"user_id": user_id}

    response = requests.get(url, params=params)

    if response.status_code != 200:
        print(f"❌ Failed to check match for user {user_id}: {response.status_code}")
        print(f"   Response: {response.text}")
        sys.exit(1)

    return response.json()


def main():
    print("=" * 60)
    print("🎯 Matchmaking Integration Test")
    print("=" * 60)
    print()

    # Step 1: Register two users
    print("Step 1: Registering users...")
    username1 = generate_random_username()
    username2 = generate_random_username()

    user1 = register_user(username1, "password123")
    user2 = register_user(username2, "password123")

    user1_id = user1["user_id"]
    user2_id = user2["user_id"]

    print()

    # Step 2: Join matchmaking queue
    print("Step 2: Joining matchmaking queue...")
    join_queue(user1_id)
    join_queue(user2_id)

    print()

    # Step 3: Wait for matchmaker to run (runs every 2 seconds)
    print("Step 3: Waiting for matchmaker to process queue...")
    wait_time = 4  # Wait 4 seconds to ensure at least one tick
    for i in range(wait_time, 0, -1):
        print(f"   ⏳ Waiting {i} seconds...")
        time.sleep(1)

    print()

    # Step 4: Check if both users were matched
    print("Step 4: Checking match results...")
    match1 = check_match(user1_id)
    match2 = check_match(user2_id)

    print()

    # Step 5: Verify results
    print("Step 5: Verifying match...")
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

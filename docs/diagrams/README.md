# Architecture Diagrams

These diagrams document the backend system architecture. Open `.drawio` files with [draw.io](https://app.diagrams.net/) (web), the draw.io desktop app, or the draw.io VS Code extension.

## Diagrams

### 01 - System Architecture
**File:** `01_system_architecture.drawio`

High-level overview of all 7 microservices, their ports, and how they communicate. Shows the client-to-service connections (HTTP and WebSocket), inter-service calls (e.g., Matchmaking validating tokens via Auth, Gameplay loading cards via Deck), and the shared data layer (MySQL 8.0 + Redis 7) all within the Docker Compose network.

### 02 - Game Phase Flow
**File:** `02_game_phase_flow.drawio`

State machine showing the lifecycle of a game session: `WAITING_FOR_PLAYERS` -> `PRE_TURN` (10s card selection) -> `ACTIVE` (30s combat) -> loop back or `GAME_OVER`. Includes the board layout (2x3 grid per player), elixir system details, attack priority rules, and how surrender works.

### 03 - Database ER Diagram
**File:** `03_database_er_diagram.drawio`

Entity-relationship diagram of all 12 database tables with columns, types, primary keys, and foreign key relationships. Tables are grouped by domain: Auth (users, user_sessions), Card (cards, card_stats, card_abilities, affiliations), Player (player_cards, decks, deck_cards, card_packs), and Game (matchmaking_queue, game_sessions).

### 04 - Player Journey Sequence
**File:** `04_player_journey_sequence.drawio`

UML sequence diagram tracing a player's full journey from registration through login, matchmaking queue, match found, match accept, WebSocket game session, and game end with MMR update. Shows every HTTP call and data flow between Client, Auth, Deck, Matchmaking, Gameplay, MySQL, and Redis.

### 05 - Card Effects Architecture
**File:** `05_card_effects_architecture.drawio`

Illustrates the Factory + Registry design pattern used for the card ability system. Shows the data flow from database storage (card_abilities table) through loading, registry lookup, factory resolution, and the Ability interface, down to combat execution with trigger points (summon, on_attack, on_damaged, on_death). Includes reference panels listing all 17 effect types and 10 target types.

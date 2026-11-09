# Core review

Core behaviours noticed while building frontends that may need changes. Each item says what the
core does now, where, and the question to decide. Remove an item once it's decided (and changed,
if needed).

## Diplomacy

### Tributes don't change the shown mood right away

- **Now**: a tribute (gold or a spell) raises both wizards' `StartingRelation` by 5
  (`Player.thanksForTribute` → `AdjustDiplomaticRelation`, `core/player/diplomacy.go`,
  `core/player/player.go`). The mood the diplomacy screen shows is `VisibleRelation`, which only
  drifts toward `StartingRelation` at the start of a turn, by 1-2 points and only when
  `rand.N(140) > abs(VisibleRelation)` (`Relationship.UpdateTurn`, `core/player/player.go`; its
  FIXME notes the wiki says the downward drift should happen only every 3 turns).
  So after a tribute the mood bar doesn't move, and later moves only slowly; a strong mood (near
  ±100) barely moves at all.
- **Check against the original game** (not verified): there a tribute seems to improve the
  wizard's attitude at once, by an amount that depends on the gift's value.
- **Decide**: should tributes raise `VisibleRelation` right away too, and by an amount scaled to
  the gift instead of a flat 5? `TestHostDiplomacy` now checks `StartingRelation`; it would need to
  check the visible relation if this changes.

### The other wizard always accepts treaties

- **Now**: `Player.ProposeTreaty` (`core/player/diplomacy.go`) sets a pact or alliance on both
  sides, or 50 turns of peace, with no check: the other wizard always agrees. The `Relationship`
  already has `TreatyInterest` and `PeaceInterest` (and `Hostility`, `WarningCounter`), and the AI
  doesn't look at them for proposals. Spell trades, by contrast, do depend on the other side
  (`WillTradeSpells`: a roll against `TradeInterest`).
- **Check against the original game** (not verified): there the AI answers from its relation to
  you, its personality and objective, and existing treaties, and refuses or breaks treaties.
- **Decide**: make `ProposeTreaty` ask the AI (a hook on `AIBehavior`, using relation, interest
  and personality) and return a refusal the frontend can show (a `treaty-refused` result or event),
  plus the rules for which treaties can follow which. The command already returns an error, so
  the page shows a refusal as a message without changes.

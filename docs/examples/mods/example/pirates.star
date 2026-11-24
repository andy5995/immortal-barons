# Decides pirate raids. The game calls raid() each turn a realm plays, in
# place of its own roll. It returns a list of raids, or [] for none.

def raid(realm, factions):
    # A realm holding under 100,000 jets is left alone.
    if realm.jets < 100000:
        return []
    # About one turn in four, a faction takes jets. rand(4) is 0 to 3.
    if rand(4) != 0:
        return []
    # The game day picks the faction, so each takes a turn.
    return [{"faction": day() % len(factions), "take": "jets"}]

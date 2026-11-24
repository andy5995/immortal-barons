# Changes what deserts do, realm by realm. The game calls region() whenever it
# needs this realm's desert figures, so the answer must depend only on its
# arguments and on option(): there is no rand() or day() here.

def region(realm, row):
    # Until the realm has played enough turns, deserts keep the figures in row.
    if realm.turns_played < option("farm_turns"):
        return {}
    # Afterward each desert's food_base rises by 150, before food technology
    # and the random part. Keys left out keep their value.
    return {"food_base": row.food_base + 150}

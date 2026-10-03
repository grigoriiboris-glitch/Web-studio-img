package cardtypes

import "testing"

func TestKeyPattern(t *testing.T) {
  valid := []string{"weapon", "spell_2", "npc-type", "a1"}
  invalid := []string{"", "AWeapon", "weapon type", "weapon!", "a"}
  for _, key := range valid {
    if !keyPattern.MatchString(key) { t.Errorf("expected valid key %q", key) }
  }
  for _, key := range invalid {
    if keyPattern.MatchString(key) { t.Errorf("expected invalid key %q", key) }
  }
}

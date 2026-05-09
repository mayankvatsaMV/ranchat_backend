// preferences.go — matchmaking filter types.
// SearchPreferences is ephemeral — it exists only for the duration of a search
// and is never persisted to MongoDB. Add new filter fields here as the product grows.
package models

// GenderPreference expresses what gender a searching user wants to be matched with.
type GenderPreference string

const (
	PrefAnyone GenderPreference = "anyone"
	PrefMale   GenderPreference = "male"
	PrefFemale GenderPreference = "female"
	PrefOther  GenderPreference = "other"
)

// SearchPreferences holds the filter criteria a user applies when starting a search.
// Zero values always mean "no filter / match anyone".
//
// Future expansion: add WantCountry, WantLanguage, MinAge, MaxAge, WantInterests here.
type SearchPreferences struct {
	// What gender the searching user wants their partner to be.
	WantGender GenderPreference `json:"wantGender"`

	// ── Future fields (uncomment when ready) ────────────────────────────────
	// WantCountry  string   `json:"wantCountry"`   // "" = anyone
	// WantLanguage string   `json:"wantLanguage"`  // "" = anyone
	// MinAge       int      `json:"minAge"`        // 0 = no limit
	// MaxAge       int      `json:"maxAge"`        // 0 = no limit
	// WantInterests []string `json:"wantInterests"` // nil = anyone
}

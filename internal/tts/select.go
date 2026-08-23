package tts

import "strings"

// VoicePrefs controls automatic voice selection for a provider.
// Explicit --voice always bypasses these prefs (caller responsibility).
type VoicePrefs struct {
	// Preferred is the preference pool used for random selection when
	// non-empty (after ban filtering). Empty means "use full catalog".
	// Order is not ranked priority — selection is uniform among remaining names.
	Preferred []string
	// Banned voices are excluded from automatic selection.
	Banned []string
	// Alert is the fixed voice used when --alert is set and no --voice is given.
	Alert string
}

// NormalizePrefs trims whitespace on all voice name fields.
func NormalizePrefs(p VoicePrefs) VoicePrefs {
	return VoicePrefs{
		Preferred: trimAll(p.Preferred),
		Banned:    trimAll(p.Banned),
		Alert:     strings.TrimSpace(p.Alert),
	}
}

func normalizeGrokPrefs(p VoicePrefs) VoicePrefs {
	lower := func(items []string) []string {
		if len(items) == 0 {
			return nil
		}
		out := make([]string, len(items))
		for i, item := range items {
			out[i] = strings.ToLower(item)
		}
		return out
	}
	return VoicePrefs{
		Preferred: lower(p.Preferred),
		Banned:    lower(p.Banned),
		Alert:     strings.ToLower(p.Alert),
	}
}

func trimAll(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

// Catalog returns the built-in voice list for a provider.
func Catalog(provider ProviderType) []string {
	switch provider {
	case ProviderGroq:
		return append([]string(nil), VoiceListGroq...)
	case ProviderGrok:
		return append([]string(nil), VoiceListGrok...)
	case ProviderLlmpGrok:
		// Same Grok roster, served through the LLMP gateway.
		return append([]string(nil), VoiceListGrok...)
	case ProviderMimo:
		return append([]string(nil), VoiceListMimo...)
	default:
		return append([]string(nil), VoiceListMinimax...)
	}
}

// closedCatalog is true when the built-in list is exhaustive for the provider.
// MiniMax has 300+ system voices; our list is a curated subset, so preferred
// names outside the subset are still allowed. Grok supports custom voice IDs,
// so preferred names outside the built-in roster are kept.
func closedCatalog(provider ProviderType) bool {
	switch provider {
	case ProviderGroq, ProviderMimo:
		return true
	default:
		return false
	}
}

// DefaultAlertVoice returns the hard-coded alert voice for a provider.
func DefaultAlertVoice(provider ProviderType) string {
	switch provider {
	case ProviderGroq:
		return "daniel"
	case ProviderGrok:
		return "rex"
	case ProviderLlmpGrok:
		return "rex"
	case ProviderMimo:
		return "mimo_default"
	default:
		return "Deep_Voice_Man"
	}
}

// SelectVoice picks a voice for synthesis.
// If explicit is non-empty it is returned unchanged (CLI override).
// When alert is true, uses prefs.Alert or DefaultAlertVoice.
// Otherwise random-picks from the effective pool (preferred ∩ ¬banned, or catalog ∩ ¬banned).
// If the pool is empty (e.g. every catalog voice is banned), falls back to
// DefaultAlertVoice when that voice is not banned; otherwise the first preferred
// entry, then a fixed last-resort default so synthesis can still run.
func SelectVoice(provider ProviderType, prefs VoicePrefs, alert bool, explicit string) string {
	prefs = NormalizePrefs(prefs)
	if provider == ProviderGrok || provider == ProviderLlmpGrok {
		// Case-insensitive IDs: normalize prefs so bans/preferred match the catalog.
		prefs = normalizeGrokPrefs(prefs)
	}
	if explicit != "" {
		v := strings.TrimSpace(explicit)
		if provider == ProviderGrok || provider == ProviderLlmpGrok {
			return strings.ToLower(v)
		}
		return v
	}
	if alert {
		if prefs.Alert != "" {
			return prefs.Alert
		}
		return DefaultAlertVoice(provider)
	}

	pool := EffectivePool(provider, prefs)
	if len(pool) > 0 {
		return pool[randomSource.Intn(len(pool))]
	}

	// Pool empty: every candidate was banned. Prefer a non-banned alert default.
	def := DefaultAlertVoice(provider)
	banned := toSet(prefs.Banned)
	if _, hit := banned[def]; !hit {
		return def
	}
	// Even the default is banned — use it anyway rather than invent a new ID.
	return def
}

// EffectivePool builds the list of voices eligible for automatic selection.
//
// Rules:
//  1. Start from Preferred if non-empty; otherwise the provider catalog.
//  2. For closed-catalog providers (groq, mimo), drop preferred names not in the catalog.
//  3. Drop any voice listed in Banned (case-sensitive exact match after trim).
//  4. If preferred was set and the pool is empty after bans/catalog filter,
//     fall back to catalog minus banned.
//  5. May return empty if the catalog is entirely banned (caller must handle).
func EffectivePool(provider ProviderType, prefs VoicePrefs) []string {
	prefs = NormalizePrefs(prefs)
	catalog := Catalog(provider)
	banned := toSet(prefs.Banned)

	var base []string
	if len(prefs.Preferred) > 0 {
		base = prefs.Preferred
		if closedCatalog(provider) {
			base = intersectCatalog(base, catalog)
		}
	} else {
		base = catalog
	}

	pool := filterOut(base, banned)
	if len(pool) > 0 {
		return pool
	}

	// Preferred wiped out (all banned or none in catalog) — try full catalog minus bans.
	if len(prefs.Preferred) > 0 {
		return filterOut(catalog, banned)
	}
	// Catalog was the base and everything was banned.
	return nil
}

func intersectCatalog(preferred, catalog []string) []string {
	cat := toSet(catalog)
	out := make([]string, 0, len(preferred))
	for _, p := range preferred {
		if _, ok := cat[p]; ok {
			out = append(out, p)
		}
	}
	return out
}

func toSet(items []string) map[string]struct{} {
	s := make(map[string]struct{}, len(items))
	for _, item := range items {
		if item == "" {
			continue
		}
		s[item] = struct{}{}
	}
	return s
}

func filterOut(items []string, banned map[string]struct{}) []string {
	if len(banned) == 0 {
		return append([]string(nil), items...)
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if _, hit := banned[item]; !hit {
			out = append(out, item)
		}
	}
	return out
}

// DefaultProviderPriority is used when neither --provider / TTS_PROVIDER nor
// config provider_priority is set. First known name wins.
// Order: LLMP Grok (gateway) → Grok (xAI) → MiMo (Xiaomi) → MiniMax.
var DefaultProviderPriority = []string{
	string(ProviderLlmpGrok),
	string(ProviderGrok),
	string(ProviderMimo),
	string(ProviderMinimax),
}

// ProviderCandidates returns providers to try in order.
// Explicit provider → single-element slice (no fallback).
// Otherwise → deduplicated provider_priority (or DefaultProviderPriority).
// Unknown names are skipped. Always returns at least one known provider.
func ProviderCandidates(explicit string, priority []string) []ProviderType {
	explicit = strings.TrimSpace(explicit)
	if explicit != "" {
		return []ProviderType{normalizeProviderName(explicit)}
	}
	if len(priority) == 0 {
		priority = DefaultProviderPriority
	}
	seen := make(map[ProviderType]struct{})
	var out []ProviderType
	for _, p := range priority {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		pt := normalizeProviderName(p)
		switch pt {
		case ProviderGroq, ProviderGrok, ProviderMinimax, ProviderMimo, ProviderLlmpGrok:
			if _, ok := seen[pt]; ok {
				continue
			}
			seen[pt] = struct{}{}
			out = append(out, pt)
		}
	}
	if len(out) == 0 {
		return []ProviderType{ProviderLlmpGrok}
	}
	return out
}

// ResolveProvider picks the default provider when the user did not specify one.
// priority is an ordered list (first wins). When empty, DefaultProviderPriority
// is used (llmp-grok → grok → mimo → minimax).
// explicit comes from --provider or TTS_PROVIDER (already resolved by the flag package).
// Aliases: "xiaomi" → mimo, "llmp"/"llmp_grok" → llmp-grok.
func ResolveProvider(explicit string, priority []string) ProviderType {
	return ProviderCandidates(explicit, priority)[0]
}

func normalizeProviderName(name string) ProviderType {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "xiaomi", "mimo":
		return ProviderMimo
	case "llmp", "llmp_grok", "llmp-grok":
		return ProviderLlmpGrok
	case "grok":
		return ProviderGrok
	case "groq":
		return ProviderGroq
	case "minimax":
		return ProviderMinimax
	default:
		return ProviderType(name)
	}
}

package mcpproxy

// defaultAllowlist maps server name → set of tool names that are
// considered read-only for that upstream and so safe to expose to
// sandboxed agents by default. The user's override file may
// shrink, expand, or replace this on a per-server basis.
//
// Servers absent from this map have no built-in default; they need
// an explicit override entry to be exposed at all. The "demesne"
// server is intentionally absent to avoid a self-loop (and is
// dropped at discovery time anyway).
//
// When adding entries: verify upstream-by-upstream that listed
// tools have no side effects on external systems.
var defaultAllowlist = map[string]map[string]struct{}{
	"alignment": setOf(
		"get_article",
		"get_recommendations",
		"get_similar_articles",
		"list_disliked",
		"list_liked",
		"list_unreviewed",
		"search_articles",
		"semantic_search",
	),
	"amazon": setOf(
		"get_product_details",
		"list_regions",
		"search_products",
	),
	"anki": setOf(
		"collection_stats",
		"findNotes",
		"getTags",
		"get_cards",
		"get_due_cards",
		"modelFieldNames",
		"modelNames",
		"modelStyling",
		"notesInfo",
		"review_stats",
	),
	"bunpro": setOf(
		"get_decks",
		"get_grammar_point",
		"get_grammar_srs_details",
		"get_jlpt_progress",
		"get_review_activity",
		"get_review_forecast",
		"get_srs_overview",
		"get_stats",
		"get_study_queue",
		"get_user",
		"get_vocab",
		"get_vocab_srs_details",
	),
	"manifold": setOf(
		"get_baseline",
		"get_comments",
		"get_market",
		"get_me",
		"get_portfolio_pnl",
		"get_positions",
		"get_user",
		"list_bets",
		"search_markets",
	),
	"supermarkets-uk": setOf(
		"browse_categories",
		"compare_prices",
		"get_basket",
		"get_order_history",
		"get_product_details",
		"list_supermarkets",
		"search_products",
	),
	"wanikani": setOf(
		"get_assignments",
		"get_level_progressions",
		"get_review_statistics",
		"get_subjects",
		"get_summary",
		"get_user",
	),
	"workflowy": setOf(
		"get_node",
		"list_children",
		"list_targets",
		"search_nodes",
	),
}

func setOf(names ...string) map[string]struct{} {
	out := make(map[string]struct{}, len(names))
	for _, n := range names {
		out[n] = struct{}{}
	}
	return out
}

# Appearance-Review Lens Reference

This is the lens reference for the `sandbox-appearance-review` skill. It defines the discrete review *lenses* the pipeline's parallel reviewer agents fan out across. Each lens is scoped tightly enough that one agent can own it without overlapping neighbours; agents receive a matrix of screenshots (screen × state × viewport × colour-scheme, plus CVD/halation simulations) and emit structured findings, which a merge pass deduplicates and turns into prioritised appearance-improvement proposals.

The procedure briefs each reviewer with one lens section verbatim, so each section is self-contained: what it judges, concrete screenshot-checkable criteria, and the frameworks/thresholds it draws on. Appendix A is the deduplicated lens roster, the overlap rules (what each agent must NOT re-flag), and the merge-pass priority order. Appendix B gives the CVD and halation simulation recipes the capture stage applies.

**Emphasis:** aesthetic excellence, not bare-minimum accessibility compliance. Where thresholds are cited, treat pass/fail as the floor, not the ceiling. This skill is scoped to **visual** review — only what is judgeable from a rendered screenshot, never keyboard/ARIA/screen-reader runtime behaviour.

**Reviewer calibration — you read better than your users.** You can resolve small, thin, low-contrast, and serif text far more clearly than the human visual system, and *far* more clearly than users with low vision, astigmatism, or colour-vision deficiency. **Do not judge readability by whether you can read it.** Where a criterion gives a numeric floor — text size in px, contrast as a WCAG ratio or APCA Lc — apply it as a hard rule and raise a finding on any violation *even when the text looks perfectly legible to you*. Estimate sizes from the viewport width and known element proportions; estimate contrast from sampled or inferred colours. When a value sits near a floor, flag it rather than passing it — borderline-by-eye is below-floor for a real user. This applies most to text size (Lens 1), contrast (Lens 4), and dark-mode halation (Lens 4) — the failures your own acuity hides.

---

## Lens 1 — Visual Hierarchy & Typography

### What it judges
The clarity and intentionality of information priority communicated through type alone — scale, weight, colour, spacing, and rhythm — without reference to layout structure or colour palette decisions (those belong to Lenses 3 and 4).

### Screenshot-checkable criteria

1. **Absolute minimum text size (hard floor).** Estimate the rendered px of every text run from the viewport width and known element proportions. Floors: **body copy ≥16px**; secondary/caption/label text **≥14px**; data-dense tables may compress to **13px** but no persistent text goes **below 12px** — sub-12px text is a **critical** finding. Judge by estimated size, not by whether the text is legible to you: a 13px body paragraph that reads fine to you is still a finding.

2. **Serif and slab faces at small size.** Serif and slab-serif typefaces rely on fine stroke modulation and bracketed terminals that disintegrate below ~18px on screen — worse at sub-2× device-pixel-ratio and at weights under 500. Flag serif/slab used for **body text below 18px, or for any UI label, caption, badge, or dense data**, or as the **primary UI typeface**. Serif belongs to display headings, large pull quotes, and editorial long-form at ≥18px, not small functional text. (Typeface *personality* is Lens 9; this criterion is only the legibility-at-size cost.)

3. **Type scale coherence.** Identify all distinct font sizes visible. They should map to a recognisable scale (e.g. a modular ratio of 1.25× Major Third, 1.333× Perfect Fourth, or an explicit 12/14/16/20/24/32/48 ramp). Arbitrary intermediate sizes (e.g. 17px, 22px) break rhythm; flag them. *Refactoring UI* (Wathan & Schoger, refactoringui.com) recommends pre-defining 8–10 steps and never sizing outside them.

4. **Heading-to-body contrast.** The largest heading on screen should appear meaningfully larger *and* heavier than body copy. A ratio under 1.5× in size between adjacent hierarchy levels collapses the distinction. Check that `h1` > `h2` > body is immediately apparent at a glance, not just measurable.

5. **Weight contrast sufficiency.** Bold (≥600) versus normal (400) should be visually distinct at all sizes. At small sizes (< 14px) very light weights (100–300) become illegible and blur with the background — flag thin weights below 16px on non-display use.

6. **Secondary and tertiary hierarchy.** Labels, captions, metadata, and supporting text should use clearly reduced visual weight via size *or* colour lightness — not both when one suffices. Three or more hierarchy levels should be legible without relying on position alone.

7. **Line-length (measure).** Body paragraphs should fall in the 45–75 characters-per-line range (Robert Bringhurst's *The Elements of Typographic Style*). Columns narrower than ~40ch feel choppy; wider than ~80ch cause eye-tracking errors. Check by eyeballing approximate character count on a full paragraph line.

8. **Line-height (leading).** Body text leading should be 1.4–1.6× the font size. Headings can compress to 1.1–1.2× (tight headlines). Leading below 1.3× on body copy makes ascenders and descenders collide; above 1.8× dissipates paragraph unity.

9. **Typographic rhythm.** Paragraph spacing, heading margins, and list item gaps should feel like multiples of a shared baseline unit. Inconsistent spacing between adjacent paragraphs, or between a heading and the content it labels, signals missing system.

10. **Font pairing legibility.** If two typefaces coexist (e.g. serif display + sans-serif body), they should contrast in enough axes (width, construction, personality) to be clearly different, yet share enough tone to feel chosen. Two near-identical sans-serifs fighting each other is a common failure.

11. **Weight used for emphasis vs decoration.** Bold should appear only where it communicates priority, not decoratively. Scan for bold text in navigation labels, footer links, or non-critical micro-copy — these reduce the semantic weight of real emphasis elsewhere.

12. **Colour as hierarchy signal.** Darker or higher-chroma text for primary content, muted/grey for secondary. Ensure the grey steps are visually distinct — two shades that look identical at a glance collapse the hierarchy even if the hex values differ.

13. **Responsive type scale.** Across viewport breakpoints, do headings scale sensibly? A 48px H1 on desktop that stays 48px on mobile often overflows or crowds. Conversely, a 20px H1 on a 1440px-wide desktop lacks authority. The 16px body floor and 14px secondary floor (criterion 1) hold at *every* breakpoint; mobile body text that drops to 14px or below is a finding, not a responsive adaptation.

### Frameworks & thresholds
- Robert Bringhurst, *The Elements of Typographic Style* — 45–75ch measure, leading, weight conventions
- Wathan & Schoger, *Refactoring UI* — pre-defined type scale, weight-over-size hierarchy signal
- Modular scale ratios: Major Third (1.25), Perfect Fourth (1.333), Major Sixth (1.6) — typescale.com
- Apple HIG minimum body font: 11pt (~14.67px CSS); 17pt (~22.67px) for body text on iOS (developer.apple.com/design/human-interface-guidelines)
- W3C WCAG 2.2 "large text" threshold: ≥18pt (24px) or ≥14pt bold (18.67px) — w3.org/TR/WCAG22

---

## Lens 2 — Spacing Systems

### What it judges
The consistency and intentionality of white space: internal padding, inter-element gaps, section separation, and alignment to a grid. Distinct from layout composition (Lens 3) which concerns grid columns and reading paths.

### Screenshot-checkable criteria

1. **Grid adherence (4pt/8pt base unit).** Measure representative gaps between elements — button padding, card margins, form field gaps, icon-to-label distance. They should all be multiples of 4px (and ideally of 8px for major separations). Non-multiple values (e.g. 6px, 10px, 14px) indicate ad-hoc decisions. Material Design 3 mandates 8dp increments for major spacing and 4dp for sub-elements like icons (m3.material.io/foundations/layout/understanding-layout/spacing).

2. **Component internal padding consistency.** Buttons, cards, inputs, and badges should share a coherent internal padding rhythm. A button with 12px vertical padding next to an input with 10px vertical padding creates subliminal misalignment even when the elements are visually similar.

3. **Proximity grouping (Gestalt).** Related elements (label+input, heading+body, icon+label) should be visually closer to each other than to unrelated elements. Insufficient gap between groups, or excessive gap within a group, breaks the proximity signal. The rule: gap *within* a group < gap *between* groups, at every nesting level.

4. **White-space generosity.** Scan for cramped sections where elements are clearly competing for space. *Refactoring UI* notes that designs almost always benefit from more whitespace than the designer's first instinct. Check cards, modals, and sidebars first — these are common cramping sites.

5. **Density calibration.** High-density tables and dashboards are intentionally compact but should still maintain ≥4px row padding. Low-density marketing pages should have generous section spacing (≥64px vertical section separation) to let content breathe. Flag mismatches in *intended* density vs *delivered* density.

6. **Optical vs metric alignment.** Visually circular or pointed icons (arrows, checkmarks, rounded avatars) placed at metric pixel positions often *look* misaligned with rectangular text because their visual centre-of-mass is offset. Check that icon-to-label vertical alignment feels correct, not just pixel-perfect.

7. **Edge alignment consistency.** Text baselines, component left edges, and section tops should line up to consistent column guides. Mixed left-edge alignments (some at 16px, some at 24px from screen edge) without clear intent look unpolished. Use a mental vertical ruler scan down the left edge.

8. **Consistent component-to-component gaps.** When the same component (e.g. card, list item) repeats vertically, gaps between instances should be identical — not approximately identical. Variable gaps in a list are one of the most visible spacing defects in screenshots.

9. **Section separation clarity.** Major page sections should be separated by at least 2× the within-section gap. Sections that bleed together without adequate separation force users to discover groupings that should be obvious.

10. **Negative space as deliberate design.** Empty areas on a page or screen should feel purposeful — widening focus on surrounding content — rather than accidental underfill. If a section has noticeably more empty space than neighbours without creating a focal effect, flag it.

11. **Touch-target spacing.** Interactive elements (buttons, links, form controls) should have minimum 8dp / 8px clearance between tap targets. Apple HIG minimum touch target is 44×44pt with ≥8pt separation; Material Design recommends 48×48dp with 8dp between targets (m3.material.io).

### Frameworks & thresholds
- Material Design 3 spacing spec — 4dp/8dp base grid (m3.material.io/foundations/layout/understanding-layout/spacing)
- Apple Human Interface Guidelines — 44×44pt touch targets, 8pt separation (developer.apple.com/design/human-interface-guidelines/layout)
- Gestalt Laws of Proximity and Similarity — elements grouped by closeness communicate relationship
- *Refactoring UI* — start with too much white space and reduce; define spacing scale in advance
- 8pt Grid System (Zack MacTavish, "Designing in the 8pt grid system", Medium) — all spacing multiples of 8, sub-elements multiples of 4

---

## Lens 3 — Layout & Composition

### What it judges
The macro-level structure of the page or screen: grid adherence, visual balance, focal points, reading path, content-to-chrome ratio, and how the layout holds up across viewport breakpoints.

### Screenshot-checkable criteria

1. **Column grid adherence.** Identify the underlying column grid (12-col, 10-col, etc.). Content blocks should begin and end on column boundaries or recognised gutters. Content that straddles grid boundaries without justification (e.g. a card that's 5.3 columns wide) signals a layout not designed to a system.

2. **Visual balance.** Divide the composition into quadrants. Does any quadrant carry dramatically more visual weight (dense text, dark fills, large images) than others without a counterbalancing element? Asymmetric balance can be intentional and elegant; *unresolved* imbalance feels unstable.

3. **Focal point clarity.** Every screen should have a clear primary focal point — the element that draws the eye first. It is established through size, contrast, position (top-left has the most pull in LTR layouts), or negative space isolation. Screens without a focal point feel flat; multiple competing focal points feel chaotic.

4. **Reading path (Z/F pattern fidelity).** For marketing pages, the eye moves Z-pattern: top-left → top-right → diagonal → bottom-left → bottom-right. For content-heavy pages, F-pattern dominates. Check whether high-priority content (primary CTA, key value prop) falls along the expected scan path or is buried off-path.

5. **Content-to-chrome ratio.** "Chrome" = navigation, headers, footers, sidebars, borders, decorative elements. The ratio of chrome to useful content should feel proportionate to the page's purpose. Dashboards and content apps should maximise content area; chrome consuming >30% of screen area warrants scrutiny.

6. **Responsive reflow quality.** Compare desktop and mobile screenshots of the same screen. Check: (a) do columns stack gracefully — no overflow or horizontal scroll? (b) do images scale and crop appropriately without awkward aspect ratios? (c) does navigation collapse to an appropriate pattern? (d) does type size remain legible?

7. **Grid consistency within components.** Card grids, product grids, navigation grids — items should align both horizontally (tops/bottoms) and vertically (left/right edges) unless explicit masonry is intended. Accidental misalignment in a 3-column card grid is one of the most common and visible layout defects.

8. **Hero and full-bleed section quality.** Full-width sections (hero banners, feature callouts) should have clear internal vertical rhythm and avoid feeling either too cramped (content touching viewport edges) or too sparse (large blank zones that don't serve a compositional purpose).

9. **Sidebar and split-layout proportions.** If a sidebar exists, does its width ratio feel considered? Common ratios: 1:4 (narrow utility nav), 1:3 (document sidebar), golden ratio 1:1.618 (editorial). Mismatched proportions make one column feel uncomfortably dominant or vanishingly thin.

10. **Tension and visual interest.** Fully centred, fully symmetrical layouts with no asymmetric elements can feel static and corporate. Is there deliberate use of tension — offset typography, partial image bleeds, overlapping elements — that gives the design energy? Or conversely, is there inappropriate tension (accidental asymmetry) making the layout feel unintentional?

11. **Whitespace as composition tool.** Large blank areas should direct attention toward adjacent content. If whitespace is present but not serving a focal or breathing purpose, it likely represents missing content or failed layout planning.

### Frameworks & thresholds
- Material Design 3 responsive layout grid: 4-column (compact), 8-column (medium), 12-column (expanded) — m2.material.io/design/layout/responsive-layout-grid.html
- F-pattern and Z-pattern eye-tracking research (Nielsen Norman Group)
- Golden ratio (1:1.618) for split layout proportions
- Robert Bringhurst, *The Elements of Typographic Style* — whitespace as typographic tool
- Content-to-chrome ratio heuristic — Edward Tufte's data-ink ratio principle (*The Visual Display of Quantitative Information*)

---

## Lens 4 — Colour & Contrast

### What it judges
The full spectrum of colour decisions: palette coherence, semantic colour use, perceptual contrast (WCAG 2 AND APCA), optical discomfort phenomena (halation, vibrating complements, saturation fatigue), dark-mode-specific surface layering, and colour-blindness suitability — verifiable from screenshots without runtime access.

### Screenshot-checkable criteria

**Compute contrast, don't eyeball it.** APCA Lc and WCAG ratios are *calculated* from colour values, never judged by appearance — you will read combinations that fail. Sample or infer the foreground/background colours and report the value. The most common silent failure is **muted/grey secondary text** (captions, placeholders, metadata, timestamps): grey-on-white and mid-grey-on-dark routinely fall below Lc 75 / 4.5:1 while looking "subtle and fine" — treat all grey secondary text as suspect until its value clears the floor. **Compounding case:** small + thin + serif + low-contrast is the worst combination for astigmatic and low-vision users; when two or more of those stack on the same text, escalate the severity.

#### A. Palette Cohesion

1. **60-30-10 proportion rule.** The dominant neutral/background colour should occupy ~60% of visible area, the secondary accent ~30%, and the emphasis/CTA colour ~10%. Significant departures produce either monochromatic boredom or visual chaos. Scan the screenshot and estimate gross colour coverage proportions.

2. **Palette size and coherence.** Count distinct hue families visible. A well-designed palette has 1–2 neutrals, 1–2 brand colours, and 1–3 semantic colours (success green, warning amber, error red, info blue). More than 6–7 distinct hue families without clear structure signals a palette that has grown by accretion.

3. **Semantic colour consistency.** Red/orange = destructive or error. Green = success or positive. Blue = informational or primary. Yellow/amber = warning. These mappings should be consistent throughout the interface — if red is used both for "delete" and as a brand accent on the header, it undermines the semantic signal.

4. **Tint/shade system.** Within a single hue (e.g. brand blue), shades used should derive from a consistent lightness ramp — ideally the same system (e.g. Tailwind's 50–950 scale or Material's 0–100 tonal palette). Ad-hoc shades that don't belong to the ramp look off even when passing contrast checks.

#### B. Contrast — WCAG 2.x

5. **WCAG 2.2 text contrast (SC 1.4.3/1.4.6).** Normal text (<18pt / <14pt bold) requires 4.5:1 minimum (Level AA) or 7:1 (Level AAA). Large text (≥18pt or ≥14pt bold) requires 3:1 (AA) or 4.5:1 (AAA). UI components and graphical elements require 3:1 (SC 1.4.11). These are the legal compliance floor — *not* the aesthetic target.

6. **Non-text contrast (icons, borders, input outlines).** Form field borders, icon strokes, focus rings, and graphical elements need 3:1 against adjacent background. A light grey border on a white card often fails this silently.

#### C. Contrast — APCA (perceptual, aesthetics-grade)

7. **APCA Lc thresholds for body text.** APCA (Advanced Perceptual Contrast Algorithm, git.apcacontrast.com) reports Lightness Contrast as Lc values 0–106. Key thresholds: **Lc 75 minimum for body text**; **Lc 90 preferred for columns of body text**; Lc 60 roughly approximates WCAG 4.5:1 on light backgrounds (but APCA accounts for polarity — dark-on-light vs light-on-dark produce different Lc for the same numerical ratio). For headings and large text, Lc 60+ is acceptable. For placeholder text, Lc 30 is the minimum for distinguishability.

8. **APCA polarity asymmetry.** WCAG 2.x applies the same ratio regardless of which colour is foreground vs background. APCA treats them differently: light text on dark requires higher Lc than dark text on light for equivalent readability. A dark mode implementation that merely inverts light-mode colours often under-delivers actual perceived contrast. Verify dark-mode screenshots separately with APCA, not WCAG ratios.

9. **Font-weight × contrast interaction.** APCA's lookup table (readtech.org/ARC) pairs Lc requirements with font weight and size. A 400-weight 16px font needs Lc ≥75; the same size at 700-weight can tolerate Lc ≥60. Thin fonts (100–200 weight) at small sizes need Lc ≥90+ regardless of size. Flag thin-weight body or UI copy that would fail APCA even if it passes WCAG.

#### D. Halation & Optical Discomfort

10. **Halation on dark backgrounds (white/light text).** Pure white text on pure black (#000 on #000000) causes a "blooming" halo for viewers with astigmatism — estimated at 30–60% of the population (Level Access, levelaccess.com/blog/accessibility-for-people-with-astigmatism). The white pixels irradiate into surrounding black, making letterforms appear to smear or glow. Check: are dark-mode surfaces using near-black (e.g. #121212, #1a1a2e) rather than #000000? Is text off-white (e.g. #E0E0E0, #F5F5F5) rather than #FFFFFF? A foreground/background combination that creates a WCAG ratio above ~18:1 in dark mode is *too* contrasty and likely triggers halation.

11. **Dark-mode letter-spacing compensation.** On dark backgrounds, the halation effect makes letters appear heavier and closer together than on light backgrounds. Well-crafted dark modes increase letter-spacing by +0.01em to +0.02em (the "Rule of 2%") and may use a lighter font-weight grade to compensate. Check whether body text in dark-mode screenshots feels cramped compared to light-mode.

12. **Vibrating colour complements.** Highly saturated complementary hues placed adjacent (e.g. saturated red on saturated green, or saturated blue on saturated orange) create chromatic vibration — an uncomfortable flickering sensation at the edge. Flag any button, badge, or icon where fully saturated complementary colours are adjacent without a neutral separator.

13. **Pure-colour fatigue and blue saturation.** Saturated accent colours on dark backgrounds (especially bright blue, magenta, or lime green) appear to glow or "vibrate" due to chromatic aberration in the eye's lens. Material Design 3 recommends desaturating colours in dark mode — use tonal palette step 200 (light, desaturated) rather than step 500 (bright, saturated) on dark surfaces (m3.material.io/blog/tone-based-surface-color-m3).

#### E. Dark-Mode Surface Elevation

14. **Elevation expressed via lightness.** In dark mode, higher surfaces should be *lighter* (more luminous), not via shadows. The Material Design 3 dark theme base is #121212; a card at 1dp elevation uses a ~5% white overlay; at 8dp, ~12%; at 24dp, ~16% (m2.material.io/design/color/dark-theme.html). Check that the visual layer stack communicates depth through lightness steps — flat surfaces all at the same dark tone have no depth hierarchy.

#### F. Colour-Blindness Suitability

15. **Information never conveyed by colour alone.** Scan every place where colour encodes meaning: status indicators, chart lines, form validation states, pricing tiers, alert banners. Each must have a secondary signal — icon, label, pattern, shape, or position — that conveys the same information for colour-blind users. This is not an accessibility nicety; ~8% of males have red-green CVD (deuteranomaly/deuteranopia, protanomaly/protanopia) and will miss colour-only signals.

16. **Deutan/protan simulation check.** In deuteranopia (no green cone) and protanopia (no red cone), red and green become indistinguishable. A red error badge next to a green success badge becomes two identical brownish blobs. Check error/success/warning state differentiation without colour cues. Simulation: apply the deuteranopia SVG filter matrix (see Appendix B) to the screenshot and inspect.

17. **Tritan simulation check.** In tritanopia (no blue cone), blue/purple and green/yellow become confused. Brand colours that rely on blue-purple differentiation may collapse. Less common (~0.01%) but worth checking for blue-heavy UIs.

### Frameworks & thresholds
- WCAG 2.2 SC 1.4.3 (text, 4.5:1 AA / 7:1 AAA), SC 1.4.6 (large text), SC 1.4.11 (non-text, 3:1) — w3.org/TR/WCAG22
- APCA/SAPC algorithm and Lc lookup tables — apcacontrast.com, readtech.org/ARC, git.apcacontrast.com/documentation/APCAeasyIntro.html
- Halation/irradiation research — Level Access astigmatism guide (levelaccess.com/blog/accessibility-for-people-with-astigmatism); "Why You Should Never Use Pure Black" (uxmovement.com)
- Material Design 3 dark theme — #121212 base, tone 200 for accent colours, elevation via lightness (m2.material.io/design/color/dark-theme.html; m3.material.io/blog/tone-based-surface-color-m3)
- CVD prevalence: ~8% of males (deutan/protan), ~0.01% (tritan) — Brettel, Viénot & Mollon 1997 simulation matrices
- DaltonLens SVG filter matrices (accurate Brettel 1997 implementation) — daltonlens.org/cvd-simulation-svg-filters
- 60-30-10 colour proportion rule — industry-standard interior/graphic design heuristic

---

## Lens 5 — Depth, Elevation, Shadow, Border & Radius

### What it judges
The internal consistency of the visual "layer cake" — how shadows, borders, background fills, and corner radii work together to communicate surface hierarchy and material quality. A UI with coherent elevation feels crafted; incoherent elevation looks assembled from unrelated libraries.

### Screenshot-checkable criteria

1. **Shadow scale system.** Identify all visible shadow variants. They should form a clear ordered set: no shadow (flat/ground), subtle (1–2px y, 4–8px blur), medium (4–6px y, 12–24px blur), heavy (8–12px y, 24–48px blur), ambient glow. Each step should be unambiguously different from the next. Shadows that all look approximately the same provide no depth information.

2. **Shadow colour and opacity.** Well-crafted shadows use a dark version of the background hue (not pure black) at low opacity. Pure black (`rgba(0,0,0,0.3)`) shadows on coloured surfaces look stuck-on. Check whether shadows feel embedded in the surface colour (warmer or tinted) or appear as black blobs.

3. **Elevation-to-shadow consistency.** Higher-elevation components (modals, popovers, floating action buttons) should have proportionally larger, more diffuse shadows than lower-elevation components (cards, buttons). Modal over card over background — the shadow hierarchy should reinforce the z-order immediately.

4. **Shadow in dark mode.** Shadows are near-invisible on dark backgrounds (you cannot cast a black shadow onto a near-black surface). Dark mode must express elevation through surface lightness instead (see Lens 4, criterion 14). A dark-mode screenshot where modal surfaces are the same lightness as background cards has a collapsed elevation system.

5. **Border weight consistency.** 1px borders, 2px borders, and thick decorative borders should each have a defined role and appear only in that role. A 1px light grey border on a card mixed with a 2px brand-coloured border on an adjacent form input signals inconsistent system use.

6. **Border colour semantics.** Neutral borders (dividers, card outlines, input fields) should be a consistent grey family. Semantic borders (error inputs in red, success in green, focused inputs in brand blue) should use the established semantic palette. Neutral borders appearing in brand colour, or semantic colours used as decorative dividers, blurs meaning.

7. **Corner radius consistency.** Small interactive components (buttons, badges, chips) and large containers (cards, modals, sheets) should have distinct, consistent radius values within each class. A UI with buttons at 4px radius, some cards at 4px, some at 8px, some at 12px, with no discernible system signals radius chosen per-designer-gut. Identify all distinct radius values — ideally ≤4 system values for the whole interface.

8. **Radius-to-size proportionality.** Very large components (hero cards, full-screen modals) with very small radii (2–4px) can look pinched. Very small components (chips, badges) with large radii (8px+) look like pills or circles. Check proportionality: radius should scale loosely with component size.

9. **Layering coherence.** When components overlap (popover over input, tooltip over content, drawer over page), their backgrounds, shadows, and radii should clearly signal the layer order. A flat white popover with no shadow over a flat white card is ambiguous — the reader cannot tell which is on top.

10. **Borders vs background fill as separators.** Prefer background colour differences over borders for separation where possible (fewer lines = cleaner). When borders are used, they should be light enough to be structural (guiding the eye) rather than decorative (demanding attention). A border at full brand colour creates a jarring visual weight that fights content.

11. **Inset/outline shadows for interactive states.** Focus rings and active states often use inset box-shadows or outlines. Check that these are visually consistent in width and colour — 2px solid brand colour at 3:1 contrast against the adjacent background (WCAG SC 2.4.11 Focus Appearance). A focus ring visible in one component class but invisible in another is a consistency failure.

### Frameworks & thresholds
- Material Design elevation system: 0dp (no shadow) → 1dp → 2dp → 4dp → 6dp → 8dp → 12dp → 16dp → 24dp — each level with defined shadow parameters (m2.material.io)
- Material Design 3 dark theme elevation: lightness-based rather than shadow-based (m3.material.io)
- *Refactoring UI* — shadow scale (small/medium/large) and tinted shadow colour; radius system
- WCAG 2.2 SC 2.4.11 Focus Appearance — focus indicator must have 3:1 contrast against adjacent non-focus colours
- Apple HIG — corner radius: 10pt for app icons, 13pt for widgets, consistent within component families

---

## Lens 6 — Imagery & Iconography

### What it judges
The visual coherence and quality of all non-text graphical elements: photographs, illustrations, icons, avatars, logos, and decorative graphics. This lens does not judge colour choices (Lens 4) or how elements are positioned in layout (Lens 3) — it judges the graphical elements themselves.

### Screenshot-checkable criteria

1. **Icon set consistency.** All icons in the interface should come from the same visual family — or if mixing sets, the mixing should be invisible to a non-designer. Key consistency attributes: stroke weight (1px vs 1.5px vs 2px), cap/join style (round vs square/miter), corner radius of rectangles in icons, fill vs outline treatment, presence/absence of a pixel grid. Mixing Material Icons (outlined, 24px grid) with Font Awesome (outline, 16px grid) produces immediate visual incongruity.

2. **Optical icon sizing.** A square icon and a circular icon at the same pixel dimensions (e.g. 24×24px) have very different perceived sizes — the circle looks ~10–15% smaller because corners don't contribute visual mass. Check whether circular or rounded icons have been optically compensated (made physically slightly larger) to *appear* the same size as geometric icons. Inconsistently weighted icon sizes in a toolbar or navigation look jagged even when metrically equal.

3. **Icon-to-text baseline alignment.** Icons paired with text labels should align so the icon's visual centre-of-mass sits at approximately the text's x-height midpoint, not the cap-height midpoint or the full character box midpoint. Metric centering (using CSS `align-items: center`) rarely achieves this for mixed-weight fonts — check whether icons look visually above or below their labels.

4. **Icon colour and weight appropriateness.** Icons should appear at the same visual weight as adjacent text. A 1px-stroke icon next to 700-weight text looks anemic; a solid/filled icon next to 300-weight text looks heavy. In secondary/disabled states, icon opacity or colour should reduce proportionally to adjacent text treatment.

5. **Image aspect ratio consistency.** Thumbnail images in a grid should maintain consistent aspect ratios (all 16:9, all 1:1, all 4:3). Mixed aspect ratios (some portrait, some landscape) in a uniform grid produce ragged edges and unequal whitespace around subjects. If images are cropped to fit, verify that the crop centre is appropriate — subjects' faces should not be cropped at the chin.

6. **Image quality and resolution.** Look for: pixelation (image rendered larger than its source resolution), compression artifacts (blocky JPEG artefacts in flat colour areas), unsharp edges in hero images at high DPI viewports. A 2× retina display showing 1× source art is immediately visible as blurry.

7. **Illustration/photography consistency.** If both custom illustrations and photographs are present, they should coexist deliberately — same colour treatment, consistent brightness levels, or a clear compositional separation (illustrations for empty states, photos for product shots). Random mixing of high-realism photos with flat vector illustrations creates tonal dissonance.

8. **Colour treatment consistency across images.** If images are displayed with a colour overlay, saturation treatment, or duotone effect, it should be applied consistently across all instances. An accidental mix of treated and untreated images (e.g. hero image with duotone overlay vs card thumbnails without) looks like an incomplete implementation.

9. **Empty-state and placeholder image quality.** Empty states, loading skeletons, and fallback avatars should match the illustration style and colour system. A generic grey placeholder box where an avatar or product image should be is a missed opportunity — but only a problem if the spec intended a styled placeholder.

10. **Decorative graphics not competing with content.** Background illustrations, SVG blobs, gradient overlays, and texture elements should stay clearly behind content and not create false focal points that compete with the primary CTA or navigation. Check contrast of decorative elements vs the body copy or UI controls laid over them.

11. **Logo and wordmark treatment.** The product logo should be rendered at appropriate weight — never scaled down to illegibility or up to imposing size. Horizontal clearspace around the logo should be proportional to its height (minimum 1× the cap-height of the wordmark as clearspace on each side). In dark mode, the logo should switch to a light or reversed variant, not force the light-mode logo on a dark background.

### Frameworks & thresholds
- Material Design icon guidelines — 24dp grid, 2dp stroke, rounded caps/joins (m3.material.io)
- Apple SF Symbols — weight matches text weight automatically; optical size tracks font size
- Optical icon sizing: circular icons ~10–15% larger than square icons for perceptual equivalence (common type foundry practice)
- Image resolution: minimum 2× source resolution for retina/HiDPI displays (1px CSS = 2px device pixels at 2× DPR)
- WCAG SC 1.4.5 (Images of Text) — prefer real text over rasterised text; images of text at 1× source on retina look blurry

---

## Lens 7 — Component Polish & States

### What it judges
The visual quality and completeness of individual UI components across their interactive states, as captured in the screenshot matrix. States visible in screenshots: default, hover, active/pressed, disabled, focused, loading, empty, and error. This lens judges affordance, touch-target adequacy, and state-to-state visual integrity.

### Screenshot-checkable criteria

1. **Affordance legibility.** Buttons must look clickable, inputs must look fillable, links must look traversable. Primary buttons should have sufficient fill, weight, or outline to read as interactive at a glance. Check the "squint test" — at half-resolution, does the primary CTA still feel like the most clickable element on screen?

2. **Hover state distinctness.** Hover states (captured via `:hover` screenshots) should be clearly different from default but clearly lighter than active/pressed. Common treatment: +/−10–15% lightness shift, or a subtle underline/background addition. A hover state that looks identical to default signals missing implementation; one that looks identical to active/pressed collapses the affordance model.

3. **Active/pressed state.** Interactive elements in their pressed state should appear physically depressed or inverted — darker fill, inset shadow, or scale transform of ~0.97×. The active state should be more extreme than hover, signalling direct physical contact. A button with the same appearance at hover and active feels unresponsive.

4. **Disabled state clarity.** Disabled elements must be visually distinct from enabled-but-unavailable elements and from regular text. Standard treatment: 40–50% opacity or a reduced-saturation variant. Critically, disabled elements should *not* have hover effects when moused-over (verify by checking the disabled screenshot shows no hover transform).

5. **Focus ring visibility and aesthetics.** Focus rings (for keyboard navigation) should be clearly visible on *every* focusable element in the screenshot. WCAG 2.2 SC 2.4.11 requires a minimum 2px outline with 3:1 contrast against adjacent background. Aesthetically, modern focus rings use rounded corners matching the component, offset by 2–3px from the element edge (not a tight 1px hug). A clunky `outline: 2px solid blue` browser default left unstyled is a visible craft signal.

6. **Loading/skeleton state quality.** Loading placeholders (skeleton screens) should approximate the shape and size of the content they replace — not generic grey rectangles that bear no resemblance to the final layout. Skeleton shimmer animation direction should be consistent across components. In screenshots, check that skeleton elements are sized proportionally to the real content.

7. **Empty state quality.** Empty states (no search results, no items in cart, first-run states) should be designed, not left as blank areas. A well-crafted empty state has: illustration or icon, a brief explanatory headline, and a CTA. An empty state that leaves the user staring at a blank content area is a visible gap.

8. **Error state visual integrity.** Form validation errors should appear below (or adjacent to) the relevant field with sufficient spacing to read without overlap. Error messages in red must include a non-colour signal (icon, bold text, explicit "Error:" prefix) for CVD users. The error state should persist until corrected — check that a field with an error has a clearly changed border/background (not just a text note below).

9. **Touch-target size.** All primary interactive elements (buttons, nav items, icons-as-buttons, checkboxes, radio buttons) should occupy a minimum 44×44px visual bounding box inclusive of padding — or show visual padding that implies it even if the rendered element is smaller. A 16×16px icon with no visible hit-area indication is a craft deficit and an accessibility failure. Apple HIG mandates 44pt; Material 48dp; W3C mobile guidelines suggest 44×44 CSS px minimum.

10. **State-to-state transition coherence.** Where multiple states are captured (e.g. default card vs loading card vs loaded card), the layout should be stable — no content-shift between states that changes surrounding element positions. Evaluate by mentally overlaying the same card in default vs loaded states: does the card height stay constant? Does text appearing in the loaded state shift neighbour elements?

11. **Component density appropriateness for context.** A data table with 40 rows visible on a mobile screenshot signals a responsive density problem. A settings panel with 4 items on a 1440px desktop signals wasted space. Check whether component density is calibrated to the viewport.

### Frameworks & thresholds
- WCAG 2.2 SC 2.4.11 Focus Appearance — minimum 2px, 3:1 contrast
- Apple HIG touch targets — 44×44pt (developer.apple.com/design/human-interface-guidelines/layout)
- Material Design 3 — 48×48dp interactive area, 8dp between targets (m3.material.io)
- W3C Mobile Accessibility Guidelines — minimum 44×44 CSS px touch targets
- *Refactoring UI* — disabled state opacity, loading skeleton design principles
- Nielsen Norman Group — empty state best practices (always provide next action)

---

## Lens 8 — Micro-typography

### What it judges
Fine-grained typographic decisions that are invisible in wireframes but conspicuous in production screenshots: text overflow, widows/orphans, justification artifacts, quotation marks, numeral style, hyphenation, and truncation. These details are the difference between type that looks "set" vs type that looks "styled in CSS."

### Screenshot-checkable criteria

1. **Widows and orphans.** A *widow* is a single short word or syllable left alone on the final line of a paragraph. An *orphan* is the first line of a paragraph isolated at the bottom of a column/page (less visible on web). Widows in short headings are especially conspicuous — an H2 reading "The Future of\nWork" is fine; one reading "The Future of\nDistributed\nWork" where "Work" sits alone is a widow. Check hero headings and pull quotes first.

2. **Justified text rivers.** If any text is set to `text-align: justify`, inspect for rivers — vertical channels of whitespace created by irregular word spacing. Rivers are common in narrow columns with long words. A justified column at 35ch with rivers looks worse than a left-aligned equivalent. Justified text rarely works well on web without CSS hyphenation support (`hyphens: auto`).

3. **Hyphenation quality.** When CSS `hyphens: auto` is active, check for: (a) hyphenation on proper nouns or brand names (should be suppressed with `hyphens: none` on those elements); (b) more than 2 consecutive hyphenated lines (typographic rule: ≤2 consecutive hyphens); (c) hyphens at the end of headings (aesthetically inappropriate). If no hyphenation is active in justified or narrow-column text, flag the rivers it creates.

4. **Typographic quotation marks.** Straight/dumb quotes (`"` `'`) vs curly/typographic quotes (`"` `"` `'` `'`). In a product screenshot, visible straight quotes in marketing copy, testimonials, or pull quotes signal either CMS configuration problems or template oversights. Check blockquotes, hero testimonials, and any inline quotation in editorial copy.

5. **Apostrophe style.** Same as above for apostrophes — the possessive `'` should be curly (right single quote `'`), not a straight tick. Visible in words like "it's", "don't", "user's". This is a detail that separates editorial care from template copy.

6. **Numeral style (oldstyle vs lining).** Oldstyle numerals (varying height, with descenders) integrate better in body text; lining numerals (uniform cap-height) are better in tables, labels, and UI controls. Most web-deployed fonts default to lining — the thing to flag is mixing styles: oldstyle numerals from a paragraph that bleeds into a data label with lining numerals from a different font.

7. **Ellipsis character.** The `…` ellipsis character is a single typographic entity with correct spacing, narrower than three periods `...` stacked. Check that truncated text (card titles, table cells, navigation labels) uses the proper ellipsis glyph. Three dots with incorrect spacing look like a visual glitch, especially in proportional-width serif fonts.

8. **Text truncation/overflow quality.** Truncated labels in cards, table cells, and navigation items should truncate gracefully: `text-overflow: ellipsis` at a sensible point, never mid-word for single-line truncation. Multi-line clamp (`-webkit-line-clamp`) should end at a complete word where possible. Check screenshots for text that overflows its container box (runs past the edge) or truncates so early the content is meaningless.

9. **Text overflow in constrained components.** Long user names, long product titles, long URL strings, and long email addresses frequently overflow or break unexpected layouts. Check: avatar + username combinations for overflow; breadcrumb items; table cells with free-text content; badge labels. This is a real-data simulation check — the screenshot matrix should include states with long/edge-case content.

10. **All-caps text legibility.** All-caps sections (section labels, navigation, form field labels, button text in some design systems) require +5–10% letter-spacing to remain readable. Unspaced all-caps blocks look cramped and hard to scan. Also check that all-caps is not applied to paragraphs longer than 2–3 words.

11. **Superscripts and footnote markers.** Price superscripts (`$9.99⁹⁹`), trademark symbols (`Brand™`), asterisks, and footnote markers should be properly styled as superscripts at ~65% of the base font size and vertically positioned at cap-height, not overflowing line boxes and disrupting leading.

12. **Number formatting.** Large numbers in data displays should be formatted with locale-appropriate separators (1,000 vs 1.000 vs 1 000). Unformatted large numbers (e.g. `1299384`) in a UI intended for general users signal missing number formatting.

### Frameworks & thresholds
- Robert Bringhurst, *The Elements of Typographic Style* — widows, rivers, quotation marks, hyphenation rules (≤2 consecutive hyphens, no proper-noun hyphenation)
- CSS `hyphens: auto` + `overflow-wrap: break-word` as baseline; `text-overflow: ellipsis` for truncation
- Unicode typographic punctuation: U+2018/U+2019 (single curly), U+201C/U+201D (double curly), U+2026 (ellipsis)
- All-caps letter-spacing convention: +0.05em to +0.1em (typographic standard across major design systems)
- Superscript size: ~65% of base font, positioned at cap-height (standard LaTeX and typographic convention)

---

## Lens 9 — Brand/Tone Cohesion & Overall Craft Signals

### What it judges
Whether the visual design reads as a unified, intentional artefact with a coherent personality — or as an assemblage of design decisions that each look fine in isolation but don't form a recognisable whole. This lens synthesises signals from across other lenses into a holistic quality assessment.

### Screenshot-checkable criteria

1. **Design vocabulary consistency.** Does the interface have a consistent "design language" — a recognisable set of shapes, weights, colours, and treatments? Compare three unrelated components (e.g. navigation bar, modal dialog, data table). Could they plausibly come from the same product without seeing the brand name? If each feels like it was designed by a different team with different reference points, flag vocabulary inconsistency.

2. **Personality-to-use-case fit.** The visual tone should match the product's emotional context. A healthcare product communicating "calm, trustworthy, clinical" should use muted blues/greens, generous white space, and conservative typography. A gaming product communicating "exciting, high-energy" might use vibrant colours and tight spacing. Flag mismatches: a financial product using playful, rounded, cartoon-adjacent design without clear intentionality; or a children's app using aggressive corporate typography.

3. **Motion-design signals in static frames.** Even in screenshots, good motion design leaves traces: consistent loading skeleton shape matching the loaded state, meaningful progress indicator styling, transition-friendly layout (no elements that would brutally shift between states). These traces suggest the product was designed with motion in mind, not motion bolted on.

4. **Attention to corner cases as craft signal.** Check screenshots of edge cases (error states, empty states, long content, loading). A UI where these states are designed with the same care as the primary flow signals a high-craft team. Edge states rendered as unstyled browser defaults, blank white boxes, or obvious afterthoughts signal the opposite.

5. **Consistency of brand colour across all states.** The brand primary colour should appear consistently — same hue family across buttons, focus rings, active nav items, tags, and accents. A brand blue that is slightly different in hex across components (#2563EB vs #2461E9 vs #256CEB) is invisible individually but creates a muddy aggregate. This is the kind of difference that only shows up when multiple screenshots are viewed side-by-side.

6. **Typography personality.** The typeface family communicates tone: geometric sans-serifs (Inter, Outfit, DM Sans) feel technical and modern; humanist sans-serifs (Gill Sans, Myriad, Calibri) feel approachable and warm; slab serifs (Bitter, Roboto Slab) feel authoritative and grounded; display serifs (Playfair Display, Cormorant) feel editorial and premium. Does the typeface personality match the product's intended emotional register?

7. **Density as tone.** High information density signals a productivity or data-first product. Low density with ample white space signals a premium or content-first product. The density level should match the product category convention and user mental model. A high-density dashboard UI with lots of data is expected; the same density in a meditation app would feel anxious and wrong.

8. **Delight and surprise elements.** Are there small moments of craft — a subtle micro-interaction visible in its intermediate state, a well-designed illustration in an empty state, a clever icon, a thoughtful loading message — that demonstrate the team thought beyond the functional minimum? These are "craft signals" that distinguish polished products from functional ones.

9. **Numerical label coherence.** Badges, counts, price displays, ratings, and statistics should all use consistent number formatting, the same font-weight (typically medium to semibold for data emphasis), and consistent size relative to surrounding text. Inconsistent number treatment fragments the product's data personality.

10. **Third-party component visual integration.** Many products embed third-party components (date pickers, charts, maps, rich text editors). These typically arrive with their own design language. Check whether third-party components have been themed to match the host product (colours, border radius, typography) or stand out as obvious foreign objects with default library styling.

11. **Mobile-specific brand coherence.** Mobile screenshots of the same product should feel like the same product as desktop, not a stripped-down version. Check that the mobile navigation (hamburger menu, tab bar, bottom sheet) carries the same visual identity as the desktop navigation, not a different colour scheme or typeface because a different library was used.

### Frameworks & thresholds
- Brand archetype frameworks (Carl Jung → David Aaker brand personality dimensions: Sincerity, Excitement, Competence, Sophistication, Ruggedness)
- Design token philosophy — brand consistency enforced at token level, not per-component; Style Dictionary, Theo, or design token tooling
- "Pixel-perfect as craft culture" — Figma's design community, linear.app, stripe.com, craft.do as references for high-polish craft signals
- *Refactoring UI* — design with systems, not decisions; personality through type and colour weight

---

## Lens 10 — Cross-Cell Consistency

### What it judges
Whether the same UI component, state, or pattern is visually identical (or correctly adapted) across all cells in the screenshot matrix — across viewport breakpoints (desktop/tablet/mobile), colour schemes (light/dark), and interactive states (default/hover/error/disabled). This is the regression lens: it catches drift, missing dark-mode overrides, and responsive bugs.

### Screenshot-checkable criteria

1. **Component identity across viewports.** Take a single component (e.g. the primary navigation, the main CTA button, the card component) and compare it across all viewport screenshots. It should be immediately recognisable as "the same component" — same colour, same typography weight, same border/radius treatment — even if its layout adapts (horizontal nav → hamburger). Colour or weight changes between viewports signal missing responsive style.

2. **Dark-mode colour fidelity.** For every colour in light mode, the dark-mode counterpart should be an intentional design decision, not a CSS default or OS inversion. Check: are backgrounds genuinely dark surfaces (not white with `filter: invert(1)`)? Are text colours appropriately reduced in brightness (off-white, not full white)? Are accent colours desaturated to their dark-mode tonal equivalents (Material tone 200, not tone 500)?

3. **Shadow treatment in dark vs light mode.** Shadows that are beautifully calibrated in light mode often disappear entirely in dark mode (dark shadow on dark background = invisible). Check that every component that uses shadows in light mode has an equivalent dark-mode treatment — either a lighter surface (elevation via lightness), a subtle inner border, or a visible-in-dark shadow using the `rgba(0,0,0,X)` convention at higher opacity, or a tinted highlight shadow.

4. **Focus ring in light and dark mode.** A blue focus ring at 3:1 contrast on a white background may fail entirely in dark mode where the background is near the focus ring colour. Check focus ring screenshots across both colour schemes.

5. **Icon visibility in dark mode.** Icons that are dark grey on white (#555 on #FFF, ratio ~7:1) become dark grey on dark background (#555 on #1a1a1a, ratio < 1.5:1) when dark mode is applied without icon colour inversion. Check that icon colours are updated for dark mode, not left at their light-mode hex values.

6. **Spacing stability across viewports.** The spacing system (4pt/8pt grid) should remain consistent across breakpoints — not halved for mobile out of habit. Check that the same logical relationships (label-above-input spacing, card-to-card gap, section padding) maintain proportional consistency, even if the absolute px values are responsive.

7. **Typography reflow correctness.** At each breakpoint, verify: (a) no horizontal overflow (no words clipped by viewport edge); (b) no text so small it's unreadable (<12px CSS); (c) no heading so large it causes visual imbalance relative to body text. The heading-to-body scale ratio should remain consistent across breakpoints even as absolute sizes change.

8. **State consistency across colour schemes.** Hover, focus, disabled, and error states should each have clearly defined visual treatments in *both* light and dark mode. An error input with a red border in light mode but no visible state change in dark mode (where red may be too dark to see) is a dark-mode state gap.

9. **Image and illustration treatment across schemes.** If the product uses custom illustrations, verify that illustration colours are adapted for dark mode (not just the background changing while illustrations stay light-optimised). SVG illustrations can swap colour palettes; PNG illustrations need dark-mode variants or appropriate treatment.

10. **Loading state consistency.** Skeleton screen colours must be adapted for dark mode. A light grey (#E0E0E0) skeleton shimmer on a light background is legible; the same hex on a #121212 background is invisible. Check skeleton screenshots in dark mode.

11. **Numerical and data display consistency.** Data tables, stats, and counts should render identically in their semantic meaning across all viewport and colour-scheme combinations. A badge count showing "23" on desktop and "..." on mobile (because the component doesn't truncate integers gracefully) is a cross-cell inconsistency in data representation.

12. **Navigation pattern consistency.** If the product uses different navigation patterns per viewport (top-nav on desktop, bottom tab bar on mobile, drawer on tablet), each pattern should carry the same active-item colour, same font weight for labels, and the same spacing-from-edge convention within its pattern type.

### Frameworks & thresholds
- Material Design 3 adaptive layouts — canonical layout patterns for compact/medium/expanded breakpoints (m3.material.io/foundations/layout)
- CSS custom properties (variables) as the mechanism for colour scheme adaptation — a colour in a `var(--color-surface)` that is redefined in `prefers-color-scheme: dark` is the correct implementation signal vs hard-coded hex values
- APCA dark-mode contrast guidance — verify Lc values independently for each colour scheme (apcacontrast.com)
- Regression testing: cross-cell comparison is the visual equivalent of snapshot regression testing — any unexpected pixel difference in a stable component is a finding

---

---

## Appendix A — Recommended Deduplicated Parallel-Agent Lens Set

### Final lens roster (10 lenses, one agent each)

| # | Lens | Core focus | Primary output |
|---|------|------------|----------------|
| 1 | **Visual Hierarchy & Typography** | Type scale, weight, measure, leading, rhythm | Flagged type-scale violations, measure/leading issues, weight-contrast failures |
| 2 | **Spacing Systems** | 4/8pt grid, proximity grouping, density, edge alignment | Off-grid gaps, insufficient touch-target separation, crowding/sparse imbalances |
| 3 | **Layout & Composition** | Column grid, balance, focal point, reading path, content/chrome ratio | Layout drift, reflow failures, poor focal hierarchy, chrome-heavy screens |
| 4 | **Colour & Contrast** | WCAG 2.x + APCA thresholds, halation, vibrating complements, dark-mode elevation, CVD | Contrast failures (both metrics), halation risk flags, colour-only information, CVD simulation failures |
| 5 | **Depth / Elevation / Shadow / Border / Radius** | Shadow scale, border consistency, radius system, layering coherence | Shadow hierarchy collapse, dark-mode elevation missing, radius inconsistency |
| 6 | **Imagery & Iconography** | Icon set consistency, optical sizing, image quality, illustration tone | Mixed icon families, pixelation, optical misalignment, illustration tone mismatch |
| 7 | **Component Polish & States** | Affordance, hover/active/disabled/focus/loading/empty/error states, touch targets | Missing states, unclear affordance, non-compliant focus rings, undersized targets |
| 8 | **Micro-typography** | Widows, rivers, quotation marks, truncation, overflow, numerals | Widows in headings, dumb quotes, poor truncation, overflow defects |
| 9 | **Brand / Tone Cohesion** | Design vocabulary unity, personality-fit, craft signals, third-party integration | Vocabulary inconsistency, brand-tone mismatch, unthemed third-party components |
| 10 | **Cross-Cell Consistency** | Same component across viewports × colour-schemes × states | Dark-mode missing overrides, skeleton colour failures, cross-breakpoint drift |

### Severity floors

Size-floor violations (Lens 1 criterion 1 — sub-12px persistent text) and contrast-floor violations (Lens 4 — below WCAG AA or APCA Lc 75) are **critical** regardless of whether the reviewer finds the text legible. The merge pass must not downgrade them on the grounds that the screenshot reads clearly — that judgement is exactly the one the reviewer-calibration note rules out.

### Overlap notes — what each agent must NOT re-flag

To prevent merge-phase deduplication overload, each agent should exclude these already-owned dimensions:

- **Lens 1 (Typography)** — does NOT judge colour contrast of text (→ Lens 4), spacing *between* text blocks (→ Lens 2), or truncation (→ Lens 8).
- **Lens 2 (Spacing)** — does NOT judge column grid or reading path (→ Lens 3), touch-target *visual affordance* (→ Lens 7, size only here), or icon-to-text alignment *type* (→ Lens 6).
- **Lens 3 (Layout)** — does NOT judge typography scale (→ Lens 1) or spacing values (→ Lens 2). Focuses on macro grid and composition.
- **Lens 4 (Colour & Contrast)** — does NOT judge icon set consistency (→ Lens 6) or shadow colour (→ Lens 5). Owns all contrast metric calculations and CVD suitability.
- **Lens 5 (Depth/Elevation)** — does NOT judge focus ring contrast values (→ Lens 4 for the ratio, Lens 7 for the ring's visibility as a state), or border *colour semantics* (→ Lens 4 owns semantic colour).
- **Lens 6 (Imagery/Iconography)** — does NOT judge icon colour contrast (→ Lens 4) or icon *spacing from labels* (→ Lens 2 for the gap value, Lens 6 for optical alignment quality).
- **Lens 7 (Component States)** — does NOT judge hover colour contrast (→ Lens 4) or whether spacing inside components is on-grid (→ Lens 2). Owns state completeness and affordance.
- **Lens 8 (Micro-typography)** — does NOT judge font family personality (→ Lens 9) or line-height (→ Lens 1). Owns purely the fine-grained text-setting defects.
- **Lens 9 (Brand/Tone)** — synthesises signals from all other lenses but does NOT re-measure specific thresholds. It calls a brand-level verdict, not a component-level one.
- **Lens 10 (Cross-Cell)** — does NOT re-audit individual components. It diffs components *against themselves* across the matrix. Any finding must identify which cells differ, not merely that a problem exists.

### Recommended priority order for merge pass

1. Lens 4 (Colour & Contrast) — legal exposure, widest user impact
2. Lens 7 (Component States) — task completion blockers (inaccessible focus, missing error states)
3. Lens 1 (Typography) — readability; large surface area
4. Lens 10 (Cross-Cell) — regression risk (missing dark-mode overrides often ship silently)
5. Lens 2 (Spacing) — craft signal most visible to non-designers; easy to fix
6. Lens 3 (Layout) — structural changes; higher design cost to fix
7. Lens 5 (Depth/Elevation) — polish; lower functional impact
8. Lens 6 (Imagery/Iconography) — visual quality; variable effort to fix
9. Lens 8 (Micro-typography) — fine craft; lower priority but high signal of attention to detail
10. Lens 9 (Brand/Tone) — strategic framing; low urgency but high influence on perception

---

## Appendix B — Simulating CVD and Halation on Captured Screenshots

### B.1 — Colour Vision Deficiency (CVD) Simulation

Apply matrix transforms in any of these ways to a captured PNG/WebP screenshot:

#### SVG filter method (browser-side, exact)
Inject an `<svg>` with the relevant `<feColorMatrix>` filter into the page before capture, then screenshot the page with the filter applied. Set `color-interpolation-filters="linearRGB"` on the filter for correctness.

```svg
<!-- Deuteranopia (green cone absent) — Brettel 1997 via DaltonLens -->
<svg xmlns="http://www.w3.org/2000/svg" style="display:none">
  <filter id="deuteranopia" color-interpolation-filters="linearRGB">
    <feColorMatrix type="matrix" values="
      0.3667  0.8616 -0.2283  0  0
      0.1102  0.8988 -0.0090  0  0
     -0.0044 -0.0866  1.0910  0  0
      0       0       0       1  0" />
  </filter>
</svg>

<!-- Protanopia (red cone absent) — Brettel 1997 via DaltonLens -->
<svg xmlns="http://www.w3.org/2000/svg" style="display:none">
  <filter id="protanopia" color-interpolation-filters="linearRGB">
    <feColorMatrix type="matrix" values="
      0.1121  0.8853 -0.0005  0  0
      0.1127  0.8897 -0.0001  0  0
      0.0045  0.0000  1.0019  0  0
      0       0       0       1  0" />
  </filter>
</svg>

<!-- Tritanopia (blue cone absent) — Brettel 1997 via DaltonLens -->
<svg xmlns="http://www.w3.org/2000/svg" style="display:none">
  <filter id="tritanopia" color-interpolation-filters="linearRGB">
    <feColorMatrix type="matrix" values="
      1.0159  0.1351 -0.1488  0  0
     -0.0154  0.8683  0.1448  0  0
      0.1002  0.8168  0.1169  0  0
      0       0       0       1  0" />
  </filter>
</svg>
```

Apply to the full page: `:root { filter: url(#deuteranopia); }` in an injected `<style>` tag before screenshotting.

**Reference:** DaltonLens accurate Brettel 1997 SVG filters — daltonlens.org/cvd-simulation-svg-filters

#### CSS filter shorthand (less accurate, simpler)
Chrome DevTools and Firefox DevTools support built-in CVD simulation under Rendering → Emulate vision deficiency. Use this for quick manual inspection — it uses the browser's internal matrices (Chrome's Blink implementation documented at developer.chrome.com/docs/chromium/cvd).

#### Post-process with ImageMagick
```bash
# Deuteranopia approximation on a screenshot file
convert screenshot.png \
  -color-matrix "0.3667 0.8616 -0.2283 0 0 \
                 0.1102 0.8988 -0.0090 0 0 \
                 -0.0044 -0.0866 1.0910 0 0 \
                 0 0 0 1 0" \
  screenshot-deuteranopia.png
```

#### Post-process with Python (Pillow + NumPy)
```python
import numpy as np
from PIL import Image

DEUTERANOPIA = np.array([
    [0.3667, 0.8616, -0.2283],
    [0.1102, 0.8988, -0.0090],
    [-0.0044, -0.0866, 1.0910],
])

img = np.array(Image.open("screenshot.png").convert("RGB")) / 255.0
# Apply in linear light (approximate sRGB gamma removal)
img_lin = img ** 2.2
sim_lin = img_lin @ DEUTERANOPIA.T
sim = np.clip(sim_lin ** (1/2.2), 0, 1)
Image.fromarray((sim * 255).astype(np.uint8)).save("screenshot-deuteranopia.png")
```

**Accuracy note:** For correct tritanopia simulation, the Brettel 1997 method requires a separation plane check in LMS space — the single-matrix simplification above is accurate for protan/deutan but only approximate for tritan. Use DaltonLens's reference implementation for high-accuracy tritan simulation.

**Online tools for uploading screenshots:**
- Coblis — color-blindness.com/coblis-color-blindness-simulator (supports all 8 CVD types)
- colorpickerimage.org — upload and preview locally in-browser
- DaltonLens online tool — daltonlens.org

---

### B.2 — Halation / Irradiation Simulation

Halation is the blooming of bright pixels into adjacent dark pixels, caused by optical aberration in the eye (especially pronounced with astigmatism). To simulate it on a screenshot:

#### Method 1: Gaussian blur over the bright channel
Extract the luminance layer, threshold to isolate bright regions (> ~220/255 brightness), apply a Gaussian blur (radius 2–8px represents mild-to-severe halation), then add the blurred bright layer back onto the original at 20–40% opacity.

```python
import numpy as np
from PIL import Image, ImageFilter

img = Image.open("screenshot-dark-mode.png").convert("RGB")
arr = np.array(img).astype(float)

# Extract bright channel (luminance threshold)
lum = 0.2126 * arr[:,:,0] + 0.7152 * arr[:,:,1] + 0.0722 * arr[:,:,2]
bright_mask = np.clip((lum - 180) / 75, 0, 1)  # pixels above ~180/255

# Blur the bright areas
bright_img = Image.fromarray((arr * bright_mask[:,:,None]).astype(np.uint8))
bloomed = bright_img.filter(ImageFilter.GaussianBlur(radius=4))

# Add bloom back at 35% opacity
bloomed_arr = np.array(bloomed).astype(float)
result = np.clip(arr + bloomed_arr * 0.35, 0, 255).astype(np.uint8)
Image.fromarray(result).save("screenshot-halation-sim.png")
```

#### Method 2: CSS glow filter (in-browser preview)
For white text on dark backgrounds, inject CSS to simulate the halation glow effect:
```css
/* Inject this before screenshotting to preview halation effect */
body * { text-shadow: 0 0 4px rgba(255,255,255,0.6), 0 0 8px rgba(255,255,255,0.3); }
```

This visually approximates the bloom seen by astigmatic users on pure dark backgrounds with light text.

#### What to look for in the halation simulation
- **White or light text on #000000 or very dark backgrounds** — most susceptible
- **Fine hairline fonts (100–300 weight) in light colour on dark** — strokes merge and become unreadable
- **Small light text (< 14px) on dark** — bloom radius overwhelms character width
- **Fixes:** Switch background from #000 to #121212+; switch text from #FFF to #E0E0E0 or similar; increase letter-spacing by +0.02em; consider using a heavier font weight or negative Grade variable axis

**Reference:** Level Access astigmatism and halation guide — levelaccess.com/blog/accessibility-for-people-with-astigmatism; "Why You Should Never Use Pure Black" — uxmovement.com/content/why-you-should-never-use-pure-black-for-text-or-backgrounds; APCA dark-mode contrast guidance — apcacontrast.com

---

package mjml

import (
	"fmt"
	"github.com/Notifuse/safe-mjml/internal/parser"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/Notifuse/safe-mjml/internal/testutils"
	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

// expectedCase is a case of testdata that must match its MJML reference.
type expectedCase struct {
	name string
	// warnings are the messages of the validation warnings the case must
	// produce, in document order; any other warning fails the test.
	warnings []string
}

// expectedCases are the cases of TestMJMLAgainstExpected. Every document of
// testdata is in this list, in knownFailures or in unlistedDocuments
// (TestEveryDocumentIsListed).
var expectedCases = []expectedCase{
	{name: "mj-width-unit-in-group", warnings: []string{
		"line 5 (mj-column): Attribute width has invalid value: 50 % for type Unit, only accepts (px, %) units and 1 value(s)",
		"line 10 (mj-column): Attribute width has invalid value: +50% for type Unit, only accepts (px, %) units and 1 value(s)",
		"line 11 (mj-column): Attribute width has invalid value: +25.5% for type Unit, only accepts (px, %) units and 1 value(s)",
	}},
	{name: "mj-class-empty-name"},
	{name: "mj-style-empty"},
	{name: "mj-style-content-lost"},
	{name: "mj-width-js-spaces", warnings: []string{
		"line 2 (mj-body): Attribute width has invalid value: \u00a0580px for type Unit, only accepts (px) units and 1 value(s)",
		"line 6 (mj-image): Attribute width has invalid value: \u00a097px for type Unit, only accepts (px) units and 1 value(s)",
		"line 6 (mj-image): Attribute height has invalid value: \u00a031px for type Unit, only accepts (px, auto) units and 1 value(s)",
		"line 10 (mj-column): Attribute width has invalid value: \u00a0200px for type Unit, only accepts (px, %) units and 1 value(s)",
		"line 10 (mj-text): Attribute padding has invalid value: \u00a04px for type Unit, only accepts (px, %) units and 1 to 4 value(s)",
		"line 11 (mj-column): Attribute width has invalid value: 0x10 for type Unit, only accepts (px, %) units and 1 value(s)",
		"line 12 (mj-column): Attribute width has invalid value: \u00a020% for type Unit, only accepts (px, %) units and 1 value(s)",
		"line 15 (mj-group): Attribute width has invalid value: 0x1F4 for type Unit, only accepts (px, %) units and 1 value(s)",
		"line 16 (mj-column): Attribute width has invalid value: 0X64 for type Unit, only accepts (px, %) units and 1 value(s)",
	}},
	{name: "mj-body-width-exponent", warnings: []string{"line 2 (mj-body): Attribute width has invalid value: 6e2px for type Unit, only accepts (px) units and 1 value(s)"}},
	{name: "mj-body-width-underscore", warnings: []string{"line 2 (mj-body): Attribute width has invalid value: 600_5px for type Unit, only accepts (px) units and 1 value(s)"}},
	{name: "mj-width-huge"},
	{name: "mj-hero-body-width-unit", warnings: []string{"line 2 (mj-body): Attribute width has invalid value: 600px; for type Unit, only accepts (px) units and 1 value(s)"}},
	{name: "mj-navbar-link-name-carousel-rel"},
	{name: "mj-navbar-link-in-column", warnings: []string{
		"line 5 (mj-navbar-link): mj-navbar-link cannot be used inside mj-column, only inside: mj-attributes, mj-navbar",
		"line 6 (mj-navbar-link): mj-navbar-link cannot be used inside mj-column, only inside: mj-attributes, mj-navbar",
	}},
	{name: "mj-close-tag-space", warnings: []string{"line 19 (mj-text): Attribute foo is illegal"}},
	{name: "mj-style-inline-comments"},
	{name: "mj-comment-short"},
	{name: "mj-width-unit-as-written", warnings: []string{
		"line 2 (mj-body): Attribute width has invalid value: 500PX for type Unit, only accepts (px) units and 1 value(s)",
		"line 4 (mj-column): Attribute width has invalid value: 300PX for type Unit, only accepts (px, %) units and 1 value(s)",
		"line 5 (mj-column): Attribute width has invalid value: 50 % for type Unit, only accepts (px, %) units and 1 value(s)",
		"line 8 (mj-group): Attribute width has invalid value: 80%; for type Unit, only accepts (px, %) units and 1 value(s)",
		"line 9 (mj-column): Attribute width has invalid value: 40%; for type Unit, only accepts (px, %) units and 1 value(s)",
		"line 10 (mj-column): Attribute width has invalid value: 150Px for type Unit, only accepts (px, %) units and 1 value(s)",
		"line 15 (mj-column): Attribute width has invalid value: 66.5 % for type Unit, only accepts (px, %) units and 1 value(s)",
	}},
	{name: "mjml-comment-before-root", warnings: []string{"line 7 (mj-text): Attribute foo is illegal"}},
	{name: "mj-social-element-name-exact"},
	{name: "escaped-markup-between-components"},
	{name: "mj-image-height-parseint"},
	{name: "mj-navbar-link-no-href"},
	{name: "mj-close-tag-name-junk", warnings: []string{
		"line 7 (mj-section): Attribute bogus is illegal",
		"line 9 (mj-text): Attribute bad is illegal",
		"line 13 (mj-section): Attribute worse is illegal",
	}},
	{name: "mj-comment-dashes"},
	{name: "mj-style-inline-css-class-components"},
	{name: "mj-style-inline-attribute-case"},
	{name: "mj-style-inline-cdata"},
	{name: "mj-navbar-base-url-concat"},
	{name: "mj-attribute-duplicate", warnings: []string{"line 15 (mj-text): Attribute foo is illegal"}},
	{name: "mj-head-illegal-attributes", warnings: []string{
		"line 2 (mj-head): Attribute bar is illegal",
		"line 3 (mj-title): Attribute foo is illegal",
		"line 4 (mj-preview): Attributes bar, baz are illegal",
		"line 5 (mj-attributes): Attribute qux is illegal",
	}},
	{name: "mj-ending-tag-closed-by-body", warnings: []string{"line 1 (mj-text): mj-text cannot be used inside mj-body, only inside: mj-attributes, mj-column, mj-hero"}},
	{name: "mj-ending-tag-closed-by-parent"},
	{name: "mj-ending-tag-closed-by-parent-text"},
	{name: "mj-carousel-empty"},
	{name: "mj-raw-head-outlook-conditional"},
	{name: "mj-style-inline-selectors"},
	{name: "mj-style-inline-selectors-accordion"},
	{name: "mj-style-inline-cell-attributes"},
	{name: "mj-breakpoint-unicode-close"},
	{name: "mj-text-whitespace"},
	{name: "mj-attribute-data-aria-class", warnings: []string{
		"line 4 (mj-text): Attribute data-track is illegal",
		"line 8 (mj-section): Attribute aria-label is illegal",
		"line 10 (mj-text): Attributes data-id, aria-hidden, class are illegal",
		"line 11 (mj-button): Attribute data-x is illegal",
	}},
	{name: "mj-style-inline-css-syntax"},
	{name: "mj-style-inline-cascade"},
	{name: "mj-style-inline-juice-rules"},
	{name: "mj-style-inline-cdata-raw-text"},
	{name: "mj-style-inline-comment-forms"},
	{name: "mj-style-inline-css-variables"},
	{name: "mj-style-inline-css-select"},
	{name: "mj-style-inline-css-select-edges"},
	{name: "mj-style-inline-css-select-more"},
	{name: "mj-style-inline-markup-elements"},
	{name: "mj-style-inline-raw-text-elements"},
	{name: "mj-style-inline-foreign-context"},
	{name: "mj-style-inline-close-tag-end"},
	{name: "mj-style-inline-close-tag-unclosed-quote"},
	{name: "mj-style-inline-conditionals-merged"},
	{name: "mj-style-inline-conditionals-merged-body"},
	{name: "mj-button-background-none", warnings: []string{"line 1 (mj-button): Attribute background-color has invalid value: none for type Color"}},
	{name: "mj-body-width-unitless", warnings: []string{"line 1 (mj-body): Attribute width has invalid value: 600 for type Unit, only accepts (px) units and 1 value(s)"}},
	{name: "mjml-owa-desktop"},
	{name: "mj-image-full-width", warnings: []string{"line 1 (mj-image): Attribute full-width is illegal"}},
	{name: "mj-head-duplicates"},
	{name: "mj-raw-file-start"},
	{name: "mj-social-element-unknown"},
	{name: "mj-navbar-raw"},
	{name: "mjml-dir-rtl"},
	{name: "mj-attribute-entities", warnings: []string{"line 15 (mj-divider): Attribute border-color has invalid value: #000&quot;&gt;&lt;script&gt;alert(1)&lt;/script&gt; for type Color"}},
	{name: "mj-attribute-angle-brackets"},
	{name: "mj-attribute-html-entities"},
	{name: "mj-style-inline-html-quotes"},
	{name: "mj-ending-tag-case-button"},
	{name: "mj-ending-tag-case-text"},
	{name: "mj-ending-tag-case-mixed"},
	{name: "mj-tag-case-containers"},
	{name: "mj-preamble-xml-declaration", warnings: []string{"line 8 (mj-text): Attribute foo is illegal"}},
	{name: "mj-preamble-doctype"},
	{name: "mj-preamble-bom"},
	{name: "mj-ending-tag-case-lines", warnings: []string{"line 8 (mj-text): Attribute foo is illegal"}},
	{name: "mj-tag-case", warnings: []string{"line 6 (mj-unknown): Element mj-unknown doesn't exist or is not registered"}},
	{name: "mj-style-inline-empty-own", warnings: []string{
		"line 12 (mj-text): Attribute container-background-color has invalid value:  for type Color",
		"line 13 (mj-text): Attribute padding has invalid value:  for type Unit, only accepts (px, %) units and 1 to 4 value(s)",
		"line 13 (mj-text): Attribute padding-top has invalid value:  for type Unit, only accepts (px, %) units and 1 value(s)",
		"line 14 (mj-text): Attribute padding has invalid value:  for type Unit, only accepts (px, %) units and 1 to 4 value(s)",
		"line 15 (mj-divider): Attribute container-background-color has invalid value:  for type Color",
	}},
	{name: "mj-style-inline-merge"},
	{name: "ending-tags-html-parsing"},
	{name: "ending-tags-quoted-and-comments"},
	{name: "ending-tags-close-whitespace"},
	{name: "ending-tags-in-comments"},
	{name: "ending-tags-cdata-nested"},
	{name: "attributes-button-image-table"},
	{name: "attributes-navbar-social"},
	{name: "attributes-accordion"},
	{name: "fonts-empty-and-unused"},
	{name: "fonts-in-inline-html"},
	{name: "css-class-several"},
	{name: "css-class-several-carousel"},
	{name: "ending-tags-escaped-entities"},
	{name: "ending-tags-escaped-entities-accordion"},
	{name: "ending-tags-inline-html"},
	{name: "ending-tags-inline-html-accordion"},
	// "mjml", -- MJML Badly formatted, must return an error instead
	{name: "mj-body"},
	{name: "comments-kept"},
	{name: "comments-kept-accordion"},
	{name: "comments-kept-navbar"},
	{name: "comments-kept-hero"},
	{name: "mj-wrapper-raw-first"},
	{name: "mj-section-nested-in-column", warnings: []string{
		"line 5 (mj-section): mj-section cannot be used inside mj-column, only inside: mj-attributes, mj-body, mj-wrapper",
	}},
	// The warnings TestValidationWarnings checks; MJML gives the same.
	{name: "validation-warnings", warnings: []string{
		"line 3 (mj-html-attributes): Element mj-html-attributes is not supported by safe-mjml and is ignored",
		"line 9 (mj-text): mj-text cannot be used inside mj-section, only inside: mj-attributes, mj-column, mj-hero",
		"line 11 (mj-foo): Element mj-foo doesn't exist or is not registered",
		"line 12 (mj-text): Attribute foo is illegal",
		"line 13 (mj-column): mj-column cannot be used inside mj-column, only inside: mj-attributes, mj-group, mj-section",
		"line 16 (mj-spacer): mj-spacer cannot be used inside mj-body, only inside: mj-attributes, mj-column, mj-hero",
	}},
	{name: "comments-wrapper-group-edges"},
	{name: "comments-wrapper-sections"},
	{name: "mj-wrapper-section-background-url"},
	{name: "mj-wrapper-hero"},
	{name: "mj-wrapper-fractional-body-width"},
	{name: "mj-body-background-color"},
	{name: "mj-body-background-color-empty"},
	{name: "mj-body-class"},
	{name: "mj-body-width", warnings: []string{"line 2 (mj-body): Attribute width has invalid value: 420 for type Unit, only accepts (px) units and 1 value(s)"}},
	{name: "basic"},
	{name: "comment"},
	{name: "with-head"},
	{name: "complex-layout"},
	{name: "wrapper-basic"},
	{name: "wrapper-background"},
	{name: "wrapper-fullwidth"},
	{name: "wrapper-border"},
	{name: "group-footer-test"},
	{name: "section-bg-vml-color"},
	{name: "section-fullwidth-background-image"},
	{name: "section-fullwidth-bg-transparent"},
	{name: "section-padding-top-zero"},
	{name: "austin-layout-from-mjml-io"},
	// // Austin layout component tests
	{name: "austin-header-section"},
	{name: "austin-hero-images"},
	{name: "austin-wrapper-basic"},
	{name: "austin-text-with-links"},
	{name: "austin-buttons"},
	{name: "austin-two-column-images"},
	{name: "austin-divider"},
	{name: "mj-divider"},
	{name: "mj-divider-alignment"},
	{name: "mj-divider-border"},
	{name: "mj-divider-class"},
	{name: "mj-divider-container-background-color"},
	{name: "mj-divider-in-mj-text"},
	{name: "mj-divider-padding"},
	{name: "mj-divider-width"},
	{name: "mj-divider-width-units", warnings: []string{
		"line 5 (mj-divider): Attribute width has invalid value: 200 for type Unit, only accepts (px, %) units and 1 value(s)",
		"line 6 (mj-divider): Attribute width has invalid value: 10em for type Unit, only accepts (px, %) units and 1 value(s)",
		"line 8 (mj-divider): Attribute width has invalid value:  for type Unit, only accepts (px, %) units and 1 value(s)",
		"line 9 (mj-divider): Attribute width has invalid value: -50% for type Unit, only accepts (px, %) units and 1 value(s)",
	}},
	{name: "mj-divider-container-background-transparent"},
	{name: "austin-two-column-text"},
	{name: "austin-full-width-wrapper"},
	{name: "austin-social-media"},
	{name: "austin-footer-text"},
	{name: "austin-group-component"},
	{name: "austin-global-attributes"},
	{name: "austin-map-image"},
	// Cases first taken from MRML's test suite
	{name: "mrml-divider-basic"},
	{name: "mrml-text-basic"},
	{name: "mrml-button-basic"},
	{name: "body-wrapper-section"},
	{name: "mj-attributes"},
	{name: "mj-attributes-layers", warnings: []string{"line 23 (mj-text): Attribute padding has invalid value:  for type Unit, only accepts (px, %) units and 1 to 4 value(s)"}},
	{name: "mj-class-tag-defaults"},
	{name: "mj-class-separators"},
	{name: "mj-css-class-empty"},
	{name: "mj-attributes-empty-over-layers", warnings: []string{
		"line 15 (mj-social): Attribute mode has invalid value:  for type Enum, only accepts horizontal, vertical",
		"line 18 (mj-accordion): Attribute icon-position has invalid value:  for type Enum, only accepts left, right",
	}},
	{name: "mj-all-root-and-head"},
	{name: "mj-all-align-padding-empty", warnings: []string{
		"line 5 (mj-table): Attribute align has invalid value:  for type Enum, only accepts left, right, center",
		"line 13 (mj-social): Attribute align has invalid value:  for type Enum, only accepts left, right, center",
		"line 13 (mj-social): Attribute padding-left has invalid value:  for type Unit, only accepts (px, %) units and 1 value(s)",
	}},
	{name: "mj-navbar-align-empty", warnings: []string{"line 5 (mj-navbar): Attribute align has invalid value:  for type Enum, only accepts left, center, right"}},
	// mj-group cases first taken from MRML's test suite
	{name: "mj-group"},
	{name: "mj-group-background-color"},
	{name: "mj-group-class"},
	{name: "mj-group-mso-wrapper-raw"},
	{name: "mj-group-direction"},
	{name: "mj-group-vertical-align"},
	{name: "mj-group-width"},
	{name: "mj-group-class-valign-sibling"},
	{name: "mj-group-column-before-group"},
	{name: "mj-group-decimal-percent-width"},
	{name: "mj-group-default-width-raw-sibling"},
	{name: "mj-group-default-width-sibling-column"},
	{name: "mj-group-percent-width-alone"},
	{name: "mj-group-percent-width-matching-column"},
	{name: "mj-group-percent-width-sibling-column"},
	{name: "mj-group-px-width-sibling-column"},
	{name: "mj-group-section-padding-sibling"},
	{name: "mj-group-three-siblings"},
	{name: "mj-group-two-groups"},
	{name: "mj-group-trailing-raw"},
	{name: "mj-group-raw-around"},
	{name: "mj-group-raw-between-column"},
	{name: "mj-section-raw-column"},
	{name: "mj-section-column-raw"},
	{name: "mj-section-column-raw-column"},
	{name: "mj-section-column-emptyraw-column"},
	{name: "mj-group-class-collision"},
	{name: "mj-group-attributes-px-width"},
	{name: "mj-group-mj-class-width"},
	// Simple components, first taken from MRML's test suite
	{name: "mj-button"},
	{name: "mj-button-align"},
	{name: "mj-button-background"},
	{name: "mj-button-border"},
	{name: "mj-button-border-radius"},
	{name: "mj-button-class"},
	{name: "mj-button-color"},
	{name: "mj-button-container-background-color"},
	{name: "mj-button-example"},
	{name: "mj-button-font-family"},
	{name: "mj-button-font-size"},
	{name: "mj-button-font-style"},
	{name: "mj-button-font-weight"},
	{name: "mj-button-height"},
	{name: "mj-button-href"},
	{name: "mj-button-inner-padding"},
	{name: "mj-button-line-height"},
	{name: "mj-button-padding"},
	{name: "mj-button-text-decoration"},
	{name: "mj-button-text-transform"},
	{name: "mj-button-vertical-align"},
	{name: "mj-button-width"},
	{name: "mj-button-global-attributes"},
	{name: "mj-image"},
	{name: "mj-image-src-missing-or-empty"},
	{name: "mj-image-no-src"},
	{name: "mj-image-align"},
	{name: "mj-image-border"},
	{name: "mj-image-border-radius"},
	{name: "mj-image-container-background-color"},
	{name: "mj-image-fluid-on-mobile"},
	{name: "mj-image-height"},
	{name: "mj-image-href"},
	{name: "mj-image-padding"},
	{name: "mj-image-rel"},
	{name: "mj-image-title"},
	{name: "mj-image-class"},
	{name: "mj-image-src-with-url-params"},
	{name: "mj-section"},
	{name: "mj-section-background-vml"},
	{name: "mj-background-url-empty-over-layers"},
	{name: "mj-wrapper-background-url-empty"},
	{name: "mj-section-background-color"},
	{name: "mj-section-background-url"},
	{name: "mj-section-background-url-full"},
	{name: "mj-section-body-width"},
	{name: "mj-section-border"},
	{name: "mj-section-border-radius"},
	{name: "mj-section-direction"},
	{name: "mj-section-full-width"},
	{name: "mj-section-padding"},
	{name: "mj-section-text-align"},
	{name: "mj-section-bg-cover-no-repeat"},
	{name: "mj-section-global-attributes"},
	{name: "mj-section-width"},
	{name: "mj-section-with-columns"},
	{name: "mj-section-class"},
	{name: "mj-column"},
	{name: "mj-column-background-color"},
	{name: "mj-column-border"},
	{name: "mj-column-border-issue-466"},
	{name: "mj-column-border-radius"},
	{name: "mj-column-inner-background-color"},
	{name: "mj-column-vertical-align"},
	{name: "mj-column-padding"},
	{name: "mj-column-class"},
	{name: "mj-column-global-attributes"},
	{name: "mj-column-border-order"},
	{name: "mj-column-child-cells"},
	{name: "mj-column-child-cells-carousel"},
	{name: "mj-wrapper"},
	{name: "mj-wrapper-border"},
	{name: "mj-wrapper-border-radius"},
	{name: "mj-wrapper-gap"},
	{name: "mj-wrapper-multiple-sections"},
	{name: "mj-wrapper-other"},
	{name: "mj-wrapper-padding"},
	// // MJ-Text tests
	{name: "mj-text"},
	{name: "mj-text-align"},
	{name: "mj-text-color"},
	{name: "mj-text-container-background-color"},
	{name: "mj-text-decoration"},
	{name: "mj-text-example"},
	{name: "mj-text-font-family"},
	{name: "mj-text-font-size"},
	{name: "mj-text-font-style"},
	{name: "mj-text-font-weight"},
	{name: "mj-text-class"},
	// // MJ-FONT tests
	{name: "mj-font"},
	{name: "mj-font-override-builtin"},
	{name: "mj-font-redeclared-name"},
	// // MJ-RAW tests
	{name: "mj-raw"},
	{name: "mj-content-nbsp-and-comments"},
	{name: "mj-raw-conditional-comment"},
	{name: "mj-raw-go-template"},
	// // MJ-SOCIAL tests
	{name: "mj-social"},
	{name: "mj-social-element-fallbacks", warnings: []string{"line 12 (mj-social-element): Attribute background-color has invalid value:  for type Color"}},
	{name: "mj-social-anchors"},
	{name: "mj-social-align"},
	{name: "mj-social-border-radius"},
	{name: "mj-social-class"},
	{name: "mj-social-color"},
	{name: "mj-social-complex-styling"},
	{name: "mj-social-container-background-color"},
	{name: "mj-social-element-ending"},
	{name: "mj-social-font-family"},
	{name: "mj-social-font"},
	{name: "mj-social-icon"},
	{name: "mj-social-link"},
	{name: "mj-social-mode"},
	{name: "mj-social-notifuse"},
	{name: "mj-social-padding"},
	{name: "mj-social-structure-basic"},
	{name: "mj-social-text"},
	{name: "mj-social-text-wrapper"},
	{name: "mj-social-no-ubuntu-fonts-overridden"},
	{name: "mj-social-ubuntu-fonts-with-text-content"},
	{name: "mj-social-ubuntu-fonts-icons-only-fallback"},
	// // MJ-ACCORDION tests
	{name: "mj-accordion"},
	{name: "mj-accordion-font-padding"},
	{name: "mj-accordion-icon"},
	{name: "mj-accordion-other"},
	// // MJ-NAVBAR tests
	{name: "mj-navbar"},
	{name: "mj-navbar-ico"},
	{name: "mj-navbar-align-class"},
	{name: "mj-navbar-multiple"},
	// // MJ-HERO tests
	{name: "mj-hero"},
	{name: "mj-hero-background-color"},
	{name: "mj-hero-background-height"},
	{name: "mj-hero-background-position"},
	{name: "mj-hero-background-url"},
	{name: "mj-hero-background-width"},
	{name: "mj-hero-background-empty-values", warnings: []string{
		"line 9 (mj-hero): Attribute background-width has invalid value:  for type Unit, only accepts (px, %) units and 1 value(s)",
		"line 9 (mj-hero): Attribute background-height has invalid value:  for type Unit, only accepts (px, %) units and 1 value(s)",
		"line 9 (mj-hero): Attribute background-color has invalid value:  for type Color",
	}},
	{name: "mj-hero-class"},
	{name: "mj-hero-height"},
	{name: "mj-hero-mode"},
	{name: "validation-attribute-definitions", warnings: []string{
		"line 4 (mj-text): Attribute foo is illegal",
		"line 7 (mj-qux): Element mj-qux doesn't exist or is not registered",
		"line 12 (mj-text): Attribute foo is illegal",
	}},
	{name: "mj-hero-width", warnings: []string{"line 3 (mj-hero): Attribute width is illegal"}},
	{name: "mj-hero-vertical-align"},
	{name: "mj-hero-social"},
	{name: "mj-hero-navbar"},
	{name: "mj-hero-accordion"},
	{name: "mj-hero-carousel"},
	{name: "mj-hero-padding-children"},
	{name: "mj-hero-padding-carousel"},
	{name: "mj-hero-height-padding-shorthand"},
	{name: "mj-hero-padding-no-height"},
	{name: "mj-hero-padding-taller-than-height"},
	{name: "mj-hero-fractional-body-width"},
	{name: "mj-hero-fractional-children"},
	{name: "mj-hero-fluid-height"},
	{name: "mj-carousel-image-head-styles"},
	{name: "mj-accordion-element-standalone", warnings: []string{"line 10 (mj-accordion-element): mj-accordion-element cannot be used inside mj-column, only inside: mj-accordion, mj-attributes"}},
	{name: "mj-accordion-element-standalone-icons", warnings: []string{"line 5 (mj-accordion-element): mj-accordion-element cannot be used inside mj-column, only inside: mj-accordion, mj-attributes"}},
	{name: "mj-carousel-single-image"},
	{name: "mj-carousel-four-images"},
	{name: "mj-carousel-image-standalone", warnings: []string{"line 5 (mj-carousel-image): mj-carousel-image cannot be used inside mj-column, only inside: mj-attributes, mj-carousel", "line 7 (mj-carousel-image): mj-carousel-image cannot be used inside mj-column, only inside: mj-attributes, mj-carousel"}},
	{name: "mj-head-styles-navbar-image"},
	{name: "mj-head-styles-image-navbar"},
	{name: "mj-head-styles-accordion-image"},
	{name: "mj-head-styles-carousel-image"},
	{name: "mj-head-styles-image-carousel"},
	{name: "mj-head-styles-navbar-accordion-image"},
	{name: "mj-hero-inner-attributes", warnings: []string{
		"line 11 (mj-hero): Attribute inner-background-color has invalid value:  for type Color",
		"line 14 (mj-hero): Attribute align is illegal",
		"line 14 (mj-hero): Attribute inner-padding has invalid value:  for type Unit, only accepts (px, %) units and 1 to 4 value(s)",
	}},
	{name: "mj-section-padding-shorthand-children"},
	{name: "mj-wrapper-padding-shorthand-children"},
	{name: "mj-section-border-padding-children"},
	{name: "mj-accordion-element-border"},
	{name: "mj-section-padding-attributes"},
	{name: "mj-padding-longhands-attributes"},
	{name: "mj-column-padding-children"},
	{name: "mj-column-box-children"},
	{name: "mj-image-box-width"},
	{name: "mj-group-column-widths"},
	{name: "mj-group-column-mobile-widths"},
	{name: "mj-column-empty-padding", warnings: []string{"line 1 (mj-column): Attribute padding has invalid value:  for type Unit, only accepts (px, %) units and 1 to 4 value(s)"}},
	{name: "mj-column-empty-padding-class"},
	{name: "mj-column-empty-padding-attributes", warnings: []string{"line 1 (mj-column): Attribute padding has invalid value:  for type Unit, only accepts (px, %) units and 1 to 4 value(s)"}},
	{name: "mj-group-fractional-context"},
	{name: "mj-group-pixel-column-outlook"},
	{name: "mj-group-auto-width-children"},
	{name: "mj-group-padding-children"},
	{name: "mj-carousel-fractional-width"},
	{name: "mj-body-fractional-width"},
	{name: "mj-column-width-attributes"},
	{name: "mj-image-width-capped"},
	{name: "mj-column-inner-background-no-gutter"},
	// // MJ-SPACER test
	{name: "mj-spacer"},
	{name: "mj-spacer-invalid-attributes", warnings: []string{
		"line 16 (mj-spacer): Attribute vertical-align is illegal",
		"line 18 (mj-spacer): Attribute width is illegal",
	}},
	// // MJ-TABLE tests
	{name: "mj-table"},
	{name: "mj-table-global-attributes"},
	{name: "mj-table-other"},
	{name: "mj-table-width-units", warnings: []string{"line 6 (mj-table): Attribute width has invalid value: 250.7 for type Unit, only accepts (px, %, auto) units and 1 value(s)"}},
	{name: "mj-table-table"},
	{name: "mj-table-text"},
	// // MJ-CAROUSEL tests
	{name: "mj-carousel"},
	{name: "mj-carousel-align-border-radius-class"},
	{name: "mj-carousel-icon"},
	{name: "mj-carousel-tb"},
	{name: "mj-carousel-thumbnails"},
	{name: "mj-carousel-image-attributes", warnings: []string{"line 17 (mj-carousel): Attribute tb-width has invalid value:  for type Unit, only accepts (px, %) units and 1 value(s)"}},
	{name: "mj-carousel-image-empty-class"},
	// // Custom test cases
	{name: "notifuse-open-br-tags"},
	{name: "notifuse-wrapper-bgcolor"},
	{name: "notifuse-full"},
	// // Fixtures that had a reference but were not run
	{name: "mj-breakpoint"},
	{name: "mj-breakpoint-default"},
	{name: "mj-breakpoint-options"},
	{name: "mj-breakpoint-lower"},
	{name: "mj-breakpoint-reset"},
	{name: "mj-accordion-element-font-attributes"},
	{name: "mj-social-raw"},
	{name: "mj-social-vertical-attributes"},
	{name: "mj-accordion-defaults-attributes"},
	{name: "mj-accordion-raw"},
	{name: "mj-accordion-element-repeated"},
	{name: "mj-accordion-element-attributes"},
	{name: "mj-accordion-inherited-attributes"},
	{name: "mj-accordion-padding-sides"},
	// MJML warns about the same attributes, and renders them.
	{name: "mj-accordion-child-attributes", warnings: []string{
		"line 4 (mj-accordion-text): Attribute border is illegal",
		"line 12 (mj-accordion-title): Attributes icon-align, icon-height, icon-position are illegal",
	}},
	{name: "mj-accordion-element-label-font"},
	{name: "mj-accordion-attributes-from-head"},
	{name: "mj-column-width"},
	{name: "mj-preview"},
	{name: "mj-style"},
	{name: "mj-text-height"},
	{name: "mj-text-letter-spacing"},
	{name: "mj-text-line-height"},
	{name: "mj-text-padding"},
	{name: "mj-text-transform"},
	{name: "mj-title"},
	{name: "mj-section-background-repeat-x", warnings: []string{
		"line 3 (mj-section): Attribute background-repeat has invalid value: repeat-x for type Enum, only accepts repeat, no-repeat",
		"line 8 (mj-section): Attribute background-repeat has invalid value: repeat-y for type Enum, only accepts repeat, no-repeat",
	}},
	{name: "mj-raw-between-sections"},
	{name: "mj-raw-empty-between-sections"},
	{name: "mj-section-chaining-full-width"},
	{name: "mj-wrapper-raw-between-sections"},
	{name: "mj-section-then-full-width-wrapper"},
	{name: "mj-body-width-wrapper"},
	{name: "mj-wrapper-full-width-sections"},
	{name: "mj-wrapper-delegated-background"},
	{name: "mj-wrapper-full-width-padding"},
	{name: "mj-section-full-width-false"},
	{name: "mj-wrapper-background-body-width"},
	{name: "mj-wrapper-background"},
	{name: "mj-wrapper-background-full-width"},
	{name: "mj-wrapper-background-position", warnings: []string{"line 3 (mj-wrapper): Attribute background-repeat has invalid value: repeat-x for type Enum, only accepts repeat, no-repeat"}},
}

/*
TestMJMLAgainstExpected runs a suite of integration tests to verify that the MJML rendering
implementation produces HTML output matching the expected results for a variety of MJML input files,
rendered by MJML 4.18 (tools/mjml-reference).

// AIDEV-NOTE: If you are unsure about a test output, try the htmlcompare utility for a semantic diff.
// Example:
//   cd mjml/testdata && ../../bin/htmlcompare basic
// Or from project root:
//   ./bin/htmlcompare basic --testdata-dir mjml/testdata

For each test case, it reads the corresponding MJML file from the "testdata" directory, using the test case name
(e.g., "basic") to construct the filename "testdata/basic.mjml". It then renders the MJML to HTML using the Render function,
and compares the output to a pre-generated expected HTML file named "testdata/basic.html".

The mapping is:
  - For test case "foo", the MJML input is at "testdata/foo.mjml"
  - The expected HTML output is at "testdata/foo.html"

On mismatch, the test provides a detailed DOM diff, logs style differences, and writes both
actual and expected outputs to temporary files for debugging purposes.
*/
func TestMJMLAgainstExpected(t *testing.T) {
	for _, tc := range expectedCases {
		t.Run(tc.name, func(t *testing.T) {
			expected, actual, warnings, err := renderTestCase(t, tc.name)
			if err != nil {
				t.Fatalf("Failed to render MJML: %v", err)
			}
			got := make([]string, len(warnings))
			for i, w := range warnings {
				got[i] = fmt.Sprintf("line %d (%s): %s", w.Line, w.TagName, w.Message)
			}
			if strings.Join(got, "\n") != strings.Join(tc.warnings, "\n") {
				t.Errorf("warnings:\n got: %q\nwant: %q", got, tc.warnings)
			}

			if allDifferences := referenceDifferences(expected, actual); len(allDifferences) > 0 {
				writeDebugFiles(tc.name, expected, actual)
				t.Errorf("\n=== COMPREHENSIVE DIFFERENCE ANALYSIS ===\n%s\n===========================================",
					strings.Join(allDifferences, "\n\n"))
			}
		})
	}
}

// renderTestCase reads testdata/<name>.mjml and its reference testdata/<name>.html,
// and renders the MJML with Render, giving it the carousel and navbar IDs of
// the reference (see referenceIDGenerator). It returns the reference, the
// rendered HTML and the render error; it stops the test if a file cannot be
// read or the reference is empty.
func renderTestCase(t *testing.T, name string) (expected, actual string, warnings []Warning, err error) {
	t.Helper()
	filename := getTestdataFilename(name)

	mjmlContent, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("Failed to read test file %s: %v", filename, err)
	}

	expectedFile := strings.Replace(filename, ".mjml", ".html", 1)
	expectedContent, err := os.ReadFile(expectedFile)
	if err != nil {
		t.Fatalf("Failed to read expected HTML file %s: %v", expectedFile, err)
	}
	expected = string(expectedContent)

	// FAIL LOUD AND CLEAR: Check for empty expected HTML file
	if len(strings.TrimSpace(expected)) == 0 {
		t.Fatalf("❌ EMPTY EXPECTED HTML FILE: %s\n"+
			"🚨 The expected HTML file is completely empty! This indicates:\n"+
			"   - Missing reference implementation output\n"+
			"   - Failed HTML generation during test setup\n"+
			"   - Incomplete test case preparation\n"+
			"📝 Action required: Generate valid expected HTML content for this test case\n"+
			"💡 Hint: Use the reference MJML implementation to generate expected output",
			expectedFile)
	}

	// Get actual output from Go implementation (direct library usage)
	actual, err = Render(string(mjmlContent),
		WithIDGenerator(referenceIDGenerator(expected)),
		withWarningHandler(func(w Warning) { warnings = append(warnings, w) }))
	return expected, actual, warnings, err
}

// referenceDifferences compares rendered HTML with its MJML reference and
// returns a description of every difference found (none when they match).
func referenceDifferences(expected, actual string) []string {
	// Collect ALL difference types instead of early returns for comprehensive analysis
	var allDifferences []string

	// The references are MJML 4.18's own output: both sides are compared as
	// written, with no rewriting of the markup.
	normalizedExpected := expected
	normalizedActual := actual

	// Check for MSO table attribute differences FIRST (before DOM comparison)
	// because MSO conditionals are not part of DOM and will be ignored by DOM comparison
	msoTableDiff := checkMSOTableAttributeDifferences(normalizedExpected, normalizedActual)
	if msoTableDiff != "" {
		allDifferences = append(allDifferences, "MSO table attribute differences found:\n"+msoTableDiff)
	}

	// Check for MSO conditional comment differences
	msoDiff := checkMSOConditionalDifferences(normalizedExpected, normalizedActual)
	if msoDiff != "" {
		allDifferences = append(allDifferences, "MSO conditional comment differences found:\n"+msoDiff)
	}

	// The DOM comparison parses both documents, and the parser repairs
	// missing tags (a <tr> or </td> left out passes it): compare the raw
	// sequence of tags and conditional comment markers as well.
	if diff := preambleDifference(normalizedExpected, normalizedActual); diff != "" {
		allDifferences = append(allDifferences, "Document preamble differences:\n"+diff)
	}

	if diff := displayedTextDifference(normalizedExpected, normalizedActual); diff != "" {
		allDifferences = append(allDifferences, "Displayed text differences:\n"+diff)
	}

	if diff := tagSequenceDifference(normalizedExpected, normalizedActual); diff != "" {
		allDifferences = append(allDifferences, "Tag sequence differences (before the HTML parser repairs them):\n"+diff)
	}

	// Compare outputs using DOM tree comparison
	domTreesMatch := compareDOMTrees(normalizedExpected, normalizedActual)
	reportedBeforeDOM := len(allDifferences)
	if !domTreesMatch {
		// Check for HTML entity encoding differences
		entityDiff := checkHTMLEntityDifferences(normalizedExpected, normalizedActual)
		if entityDiff != "" {
			allDifferences = append(allDifferences, "HTML entity encoding differences found:\n"+entityDiff)
		}

		// Check for VML attribute differences
		vmlDiff := checkVMLAttributeDifferences(normalizedExpected, normalizedActual)
		if vmlDiff != "" {
			allDifferences = append(allDifferences, "VML attribute differences found:\n"+vmlDiff)
		}

		// Check for background CSS property differences
		bgDiff := checkBackgroundPropertyDifferences(normalizedExpected, normalizedActual)
		if bgDiff != "" {
			allDifferences = append(allDifferences, "Background CSS property differences found:\n"+bgDiff)
		}

		// Enhanced DOM-based diff with debugging
		domDiff := createDOMDiff(normalizedExpected, normalizedActual)
		if domDiff != "" {
			allDifferences = append(allDifferences, "DOM structure differences:\n"+domDiff)
		}

		// Enhanced debugging: analyze style differences with precise element identification
		// AIDEV-NOTE: Only log style differences when they actually exist to reduce noise
		styleResult := testutils.CompareStylesPrecise(normalizedExpected, normalizedActual)
		if styleResult.ParseError != nil {
			allDifferences = append(
				allDifferences,
				fmt.Sprintf("DOM parsing failed: %v", styleResult.ParseError),
			)
		} else if styleResult.HasDifferences {
			var styleDiffs []string
			styleDiffs = append(styleDiffs, "Style differences:")
			for _, element := range styleResult.Elements {
				switch element.Status {
				case testutils.ElementExtra:
					componentInfo := ""
					if element.Component != "" {
						componentInfo = fmt.Sprintf(" [created by %s]", element.Component)
					}
					styleDiffs = append(styleDiffs, fmt.Sprintf("  Extra element[%d]: <%s class=\"%s\" style=\"%s\">%s",
						element.Index, element.Tag, element.Classes, element.Actual, componentInfo))
				case testutils.ElementMissing:
					styleDiffs = append(styleDiffs, fmt.Sprintf("  Missing element[%d]: <%s class=\"%s\" style=\"%s\">",
						element.Index, element.Tag, element.Classes, element.Expected))
				case testutils.ElementDifferent:
					componentInfo := ""
					if element.Component != "" {
						componentInfo = fmt.Sprintf(" [created by %s]", element.Component)
					}
					styleDiffs = append(styleDiffs, fmt.Sprintf("  Style diff element[%d]: <%s class=\"%s\">%s",
						element.Index, element.Tag, element.Classes, componentInfo))
					styleDiffs = append(styleDiffs, fmt.Sprintf("    Expected: style=\"%s\"", element.Expected))
					styleDiffs = append(styleDiffs, fmt.Sprintf("    Actual:   style=\"%s\"", element.Actual))
					if !element.StyleDiff.IsEmpty() {
						styleDiffs = append(styleDiffs, fmt.Sprintf("    %s", element.StyleDiff.String()))
					}
				}
			}
			if len(styleDiffs) > 0 {
				allDifferences = append(allDifferences, strings.Join(styleDiffs, "\n"))
			}
		}
	}

	// compareDOMTrees found a difference that none of the reports above names
	// (they normalize whitespace and attribute order): report the first text
	// that differs, so that the difference is not lost.
	if !domTreesMatch && len(allDifferences) == reportedBeforeDOM {
		allDifferences = append(allDifferences, "Text differences:\n"+firstTextDifference(normalizedExpected, normalizedActual))
	}

	// The slash of a void element (<br>, <br/>, <br />) is not compared: an
	// HTML parser gives the same tree for the three, and the slashless form
	// of MJML's references is html-minifier's serialization, not MJML's
	// render, which keeps ending-tag content as written. A self-closing
	// tag that changes the tree (<path/> in SVG) differs in the DOM above.

	// <style> content is compared rule by rule and declaration by declaration,
	// order included. (Until 2026-11, it was compared as a sorted multiset of
	// characters, which let swapped values and reordered rules pass.)
	if cssDiffs := styleElementDifferences(normalizedExpected, normalizedActual); len(cssDiffs) > 0 {
		allDifferences = append(allDifferences, "Stylesheet differences (<style> elements, compared rule by rule):\n  "+strings.Join(cssDiffs, "\n  "))
	}

	return allDifferences
}

// styleElementDifferences compares the <style> elements of two documents, in
// document order, rule by rule and declaration by declaration.
func styleElementDifferences(expected, actual string) []string {
	expectedDoc, err := goquery.NewDocumentFromReader(strings.NewReader(expected))
	if err != nil {
		return []string{fmt.Sprintf("cannot parse expected HTML: %v", err)}
	}
	actualDoc, err := goquery.NewDocumentFromReader(strings.NewReader(actual))
	if err != nil {
		return []string{fmt.Sprintf("cannot parse actual HTML: %v", err)}
	}
	expectedStyles := expectedDoc.Find("style")
	actualStyles := actualDoc.Find("style")

	var diffs []string
	if expectedStyles.Length() != actualStyles.Length() {
		diffs = append(diffs, fmt.Sprintf("expected %d <style> elements, got %d", expectedStyles.Length(), actualStyles.Length()))
	}
	n := min(expectedStyles.Length(), actualStyles.Length())
	for i := 0; i < n; i++ {
		diffs = append(diffs, testutils.CompareStylesheets(
			testutils.ParseStylesheet(expectedStyles.Eq(i).Text()),
			testutils.ParseStylesheet(actualStyles.Eq(i).Text()),
			fmt.Sprintf("<style> %d, ", i+1),
		)...)
	}
	return diffs
}

// getTestdataFilename returns the file path for a test MJML file located in the "testdata" directory,
// using the provided testName as the base filename. The resulting path has the format "testdata/{testName}.mjml".
func getTestdataFilename(testName string) string {
	return fmt.Sprintf("testdata/%s.mjml", testName)
}

// writeDebugFiles writes both expected and actual HTML outputs to the
// temporary directory, for diffing.
func writeDebugFiles(testName, expected, actual string) {
	dir := os.TempDir()
	os.WriteFile(filepath.Join(dir, "expected_"+testName+".html"), []byte(expected), 0o644)
	os.WriteFile(filepath.Join(dir, "actual_"+testName+".html"), []byte(actual), 0o644)
}

// TestDirectLibraryUsage demonstrates and tests direct library usage
func TestDirectLibraryUsage(t *testing.T) {
	mjmlInput := `<mjml>
		<mj-body>
			<mj-section>
				<mj-column>
					<mj-text>Direct library test</mj-text>
				</mj-column>
			</mj-section>
		</mj-body>
	</mjml>`

	// Test direct library usage as documented in the restructuring plan
	html, err := Render(mjmlInput)
	if err != nil {
		t.Fatalf("Direct library usage failed: %v", err)
	}

	// Verify basic structure
	if !strings.Contains(html, "<!doctype html>") {
		t.Error("Output should contain DOCTYPE")
	}

	if !strings.Contains(html, "Direct library test") {
		t.Error("Output should contain the text content")
	}

	if !strings.Contains(html, "mj-column-per-100") {
		t.Error("Output should contain responsive CSS classes")
	}
}

// TestComponentCreation tests the component creation pattern from the plan
func TestComponentCreation(t *testing.T) {
	mjmlInput := `<mjml><mj-body><mj-section><mj-column><mj-text>Test</mj-text></mj-column></mj-section></mj-body></mjml>`

	// Parse MJML
	ast, err := parser.ParseMJML(mjmlInput)
	if err != nil {
		t.Fatalf("ParseMJML failed: %v", err)
	}

	// Create component tree
	component, err := newComponentTree(ast)
	if err != nil {
		t.Fatalf("NewFromAST failed: %v", err)
	}

	// Render to HTML
	html, err := renderComponentString(component)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	// Verify output
	if !strings.Contains(html, "Test") {
		t.Error("Output should contain test text")
	}
}

// parseStyleProperties returns the declarations of an inline style that take
// effect (testutils.EffectiveDeclarations), so that a shorthand written after
// one of its longhands differs from the reverse order.
func parseStyleProperties(style string) map[string]string {
	return testutils.EffectiveDeclarations(style)
}

// StyleDiff represents differences between expected and actual CSS properties
type StyleDiff struct {
	Missing    map[string]string    // prop: expectedValue
	Mismatched map[string][2]string // prop: [expected, actual]
	Extra      map[string]string    // prop: actualValue
}

// IsEmpty returns true if there are no differences
func (d StyleDiff) IsEmpty() bool {
	return len(d.Missing) == 0 && len(d.Mismatched) == 0 && len(d.Extra) == 0
}

// String formats the diff for readable output
func (d StyleDiff) String() string {
	if d.IsEmpty() {
		return ""
	}

	var parts []string

	if len(d.Missing) > 0 {
		var missing []string
		for prop, value := range d.Missing {
			missing = append(missing, fmt.Sprintf("%s=%s", prop, value))
		}
		parts = append(parts, fmt.Sprintf("Missing: %s", strings.Join(missing, ", ")))
	}

	if len(d.Mismatched) > 0 {
		var mismatched []string
		for prop, values := range d.Mismatched {
			mismatched = append(mismatched, fmt.Sprintf("%s=%s→%s", prop, values[0], values[1]))
		}
		parts = append(parts, fmt.Sprintf("Wrong values: %s", strings.Join(mismatched, ", ")))
	}

	if len(d.Extra) > 0 {
		var extra []string
		for prop, value := range d.Extra {
			extra = append(extra, fmt.Sprintf("%s=%s", prop, value))
		}
		parts = append(parts, fmt.Sprintf("Extra: %s", strings.Join(extra, ", ")))
	}

	return strings.Join(parts, " | ")
}

// compareStylePropertiesMaps compares two CSS property maps directly
func compareStylePropertiesMaps(expectedProps, actualProps map[string]string) StyleDiff {
	diff := StyleDiff{
		Missing:    make(map[string]string),
		Mismatched: make(map[string][2]string),
		Extra:      make(map[string]string),
	}

	// Find properties only in expected (missing)
	for prop, expectedValue := range expectedProps {
		if actualValue, exists := actualProps[prop]; !exists {
			diff.Missing[prop] = expectedValue
		} else if actualValue != expectedValue {
			diff.Mismatched[prop] = [2]string{expectedValue, actualValue}
		}
	}

	// Find properties only in actual (extra)
	for prop, actualValue := range actualProps {
		if _, exists := expectedProps[prop]; !exists {
			diff.Extra[prop] = actualValue
		}
	}

	return diff
}

// compareDOMTrees compares two HTML strings using DOM tree comparison
// This approach handles attribute ordering, CSS property ordering, and whitespace normalization
// The content of <style> elements is left to styleElementDifferences.
func compareDOMTrees(expected, actual string) bool {
	expectedDoc, err := goquery.NewDocumentFromReader(strings.NewReader(expected))
	if err != nil {
		return false
	}

	actualDoc, err := goquery.NewDocumentFromReader(strings.NewReader(actual))
	if err != nil {
		return false
	}

	return compareNodes(expectedDoc.Selection, actualDoc.Selection)
}

// compareNodes recursively compares two goquery selections (DOM subtrees)
func compareNodes(expected, actual *goquery.Selection) bool {
	// Compare number of nodes
	if expected.Length() != actual.Length() {
		return false
	}

	// Compare each node pair
	equal := true
	expected.Each(func(i int, expectedNode *goquery.Selection) {
		if i >= actual.Length() {
			equal = false
			return
		}

		actualNode := actual.Eq(i)

		// Compare node types and tag names
		expectedTag := goquery.NodeName(expectedNode)
		actualTag := goquery.NodeName(actualNode)
		if expectedTag != actualTag {
			equal = false
			return
		}

		// For text nodes, compare text content (normalized)
		if expectedTag == "#text" {
			expectedText := comparableText(expectedNode, expectedNode.Text())
			actualText := comparableText(actualNode, actualNode.Text())
			if expectedText != actualText {
				equal = false
				return
			}
			return
		}

		// For element nodes, compare attributes
		if !compareAttributes(expectedNode, actualNode) {
			equal = false
			return
		}

		// Recursively compare children
		expectedChildren := expectedNode.Children()
		actualChildren := actualNode.Children()
		if !compareNodes(expectedChildren, actualChildren) {
			equal = false
			return
		}

		// Compare text content for elements that might have mixed content
		expectedText := comparableText(expectedNode, elementText(expectedNode))
		actualText := comparableText(actualNode, elementText(actualNode))

		// The content of <style> elements is compared rule by rule by
		// styleElementDifferences, which names the differing rule.
		if expectedTag != "style" && expectedText != actualText {
			equal = false
			return
		}
	})

	return equal
}

// firstTextDifference describes the first element, in document order, whose
// text compares differently (comparableText) in the two documents.
func firstTextDifference(expected, actual string) string {
	expectedDoc, err1 := goquery.NewDocumentFromReader(strings.NewReader(expected))
	actualDoc, err2 := goquery.NewDocumentFromReader(strings.NewReader(actual))
	if err1 != nil || err2 != nil {
		return fmt.Sprintf("  DOM parsing failed: expected=%v, actual=%v", err1, err2)
	}
	expectedNodes := expectedDoc.Find("*")
	actualNodes := actualDoc.Find("*")
	for i := range min(expectedNodes.Length(), actualNodes.Length()) {
		e, a := expectedNodes.Eq(i), actualNodes.Eq(i)
		et := comparableText(e, elementText(e))
		at := comparableText(a, elementText(a))
		if goquery.NodeName(e) != "style" && et != at {
			return fmt.Sprintf("  <%s> element %d:\n    expected: %q\n    actual:   %q", goquery.NodeName(e), i, et, at)
		}
	}
	return "  the documents differ, but no element's text does: check the attributes"
}

// lineBreak stands for a <br> in elementText.
const lineBreak = "\u2028"

// blockBoundary stands, in elementText, for a child element that is not
// inline (a div, a table…): no whitespace is displayed next to it.
const blockBoundary = "\u2029"

// elementText returns the text of node's own text children, in order, with
// lineBreak for each <br> child and blockBoundary for each block child: the
// text of its child elements is compared with them. The boundaries keep a
// word on its side of a block, and let comparableText drop the whitespace
// next to a block, which a client does not display.
func elementText(node *goquery.Selection) string {
	var b strings.Builder
	node.Contents().Each(func(_ int, child *goquery.Selection) {
		switch n := child.Get(0); {
		case n.Type == html.TextNode:
			b.WriteString(n.Data)
		case n.Type == html.ElementNode && n.Data == "br":
			b.WriteString(lineBreak)
		case n.Type == html.ElementNode && !inlineElements[n.Data]:
			b.WriteString(blockBoundary)
		}
	})
	return b.String()
}

// comparableText returns the text of node, trimmed, with each run of
// whitespace collapsed to one space, unless node is in an element whose
// whitespace counts (see keepsWhitespace).
//
// The MJML references are minified, as mjml --minify writes them: its
// html-minifier (collapseWhitespace) collapses the whitespace of text nodes
// ("A   text" becomes "A text"), which MJML's render does not do (it only
// trims the content of mj-text, mj-title…), nor safe-mjml. An email client
// displays both the same way, as HTML collapses whitespace in text. So the
// comparison collapses it on both sides. Where whitespace is displayed as
// written, a difference in it is a real one: it is compared as is. The raw
// tag sequence comparison is not affected.
func comparableText(node *goquery.Selection, text string) string {
	text = strings.TrimSpace(text)
	if keepsWhitespace(node) {
		return text
	}
	collapsed := strings.Join(strings.FieldsFunc(text, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f'
	}), " ")
	// Whitespace at the end or the start of a line is not displayed: none
	// around a <br>, which the minifier also removes, nor next to a block.
	for _, boundary := range []string{lineBreak, blockBoundary} {
		collapsed = strings.ReplaceAll(collapsed, " "+boundary, boundary)
		collapsed = strings.ReplaceAll(collapsed, boundary+" ", boundary)
	}
	return collapsed
}

// keepsWhitespace reports whether node, or one of its ancestors, displays
// its whitespace as written: <pre>, <textarea>, <script> and <style>, or an
// element whose inline style sets white-space to pre, pre-wrap, pre-line or
// break-spaces.
func keepsWhitespace(node *goquery.Selection) bool {
	if node.Length() == 0 {
		return false
	}
	for n := node.Get(0); n != nil; n = n.Parent {
		switch n.Data {
		case "pre", "textarea", "script", "style":
			return true
		}
		for _, attr := range n.Attr {
			if attr.Key != "style" {
				continue
			}
			value := testutils.EffectiveDeclarations(attr.Val)["white-space"]
			if strings.HasPrefix(value, "pre") || value == "break-spaces" {
				return true
			}
		}
	}
	return false
}

// compareAttributes compares attributes between two nodes, normalizing style attributes
func compareAttributes(expected, actual *goquery.Selection) bool {
	// Get all attributes from both nodes
	expectedAttrs := make(map[string]string)
	actualAttrs := make(map[string]string)

	// Extract expected attributes
	if expected.Length() > 0 {
		node := expected.Get(0)
		for _, attr := range node.Attr {
			switch {
			case attr.Key == "style":
				expectedAttrs[attr.Key] = normalizeStyleAttribute(attr.Val)
			case attr.Key == "class":
				expectedAttrs[attr.Key] = normalizeClassAttribute(attr.Val)
			case !strings.HasPrefix(attr.Key, "data-mj-debug"):
				expectedAttrs[attr.Key] = attr.Val
			}
		}
	}

	// Extract actual attributes
	if actual.Length() > 0 {
		node := actual.Get(0)
		for _, attr := range node.Attr {
			switch {
			case attr.Key == "style":
				actualAttrs[attr.Key] = normalizeStyleAttribute(attr.Val)
			case attr.Key == "class":
				actualAttrs[attr.Key] = normalizeClassAttribute(attr.Val)
			case !strings.HasPrefix(attr.Key, "data-mj-debug"):
				actualAttrs[attr.Key] = attr.Val
			}
		}
	}

	// Compare attribute maps
	if len(expectedAttrs) != len(actualAttrs) {
		return false
	}

	for key, expectedVal := range expectedAttrs {
		actualVal, exists := actualAttrs[key]
		if !exists || expectedVal != actualVal {
			return false
		}
	}

	return true
}

func normalizeClassAttribute(class string) string {
	if class == "" {
		return ""
	}

	parts := strings.Fields(class)
	sort.Strings(parts)
	return strings.Join(parts, " ")
}

// normalizeStyleAttribute normalizes CSS style attributes by sorting properties
func normalizeStyleAttribute(style string) string {
	if style == "" {
		return ""
	}

	// Parse CSS properties
	props := parseStyleProperties(style)

	// Sort properties by key for consistent comparison
	var keys []string
	for key := range props {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Rebuild style string with sorted properties
	var parts []string
	for _, key := range keys {
		if value := strings.TrimSpace(props[key]); value != "" {
			parts = append(parts, key+": "+value)
		}
	}

	result := strings.Join(parts, "; ")
	if result != "" && !strings.HasSuffix(result, ";") {
		result += ";"
	}

	return result
}

// createDOMDiff compares two HTML DOM strings and returns a formatted string describing their differences.
// It parses both expected and actual HTML strings, compares their structures, counts of common HTML tags,
// style attributes, and debug attributes. Differences are highlighted using ANSI color codes for readability.
// If no structural differences are found, it suggests checking text content and attribute values.
// Returns a human-readable summary of DOM differences or parsing errors.
func createDOMDiff(expected, actual string) string {
	// ANSI color codes
	red := "\033[31m"
	reset := "\033[0m"
	bold := "\033[1m"

	expectedDoc, err1 := goquery.NewDocumentFromReader(strings.NewReader(expected))
	actualDoc, err2 := goquery.NewDocumentFromReader(strings.NewReader(actual))

	if err1 != nil || err2 != nil {
		return fmt.Sprintf("DOM parsing failed: expected=%v, actual=%v", err1, err2)
	}

	var diffs []string

	// Compare document structures
	expectedHtml := expectedDoc.Find("html")
	actualHtml := actualDoc.Find("html")

	if expectedHtml.Length() != actualHtml.Length() {
		diffs = append(diffs, fmt.Sprintf("%sStructure mismatch:%s Expected %d html elements, got %d",
			bold, reset, expectedHtml.Length(), actualHtml.Length()))
	}

	// Compare specific elements that commonly differ
	// 50 most common HTML tags for comparison
	compareTags := []string{
		"a", "abbr", "address", "area", "article", "aside", "audio", "b", "base", "bdi",
		"bdo", "blockquote", "body", "br", "button", "canvas", "caption", "cite", "code", "col",
		"colgroup", "data", "datalist", "dd", "del", "details", "dfn", "dialog", "div", "dl",
		"dt", "em", "embed", "fieldset", "figcaption", "figure", "footer", "form", "h1", "h2",
		"h3", "h4", "h5", "h6", "head", "header", "hr", "html", "i", "iframe", "img",
		"input", "ins", "kbd", "label", "legend", "li", "link", "main", "map", "mark",
		"meta", "meter", "nav", "noscript", "object", "ol", "optgroup", "option", "output", "p",
		"param", "picture", "pre", "progress", "q", "rb", "rp", "rt", "rtc", "ruby",
		"s", "samp", "script", "section", "select", "small", "source", "span", "strong", "style",
		"sub", "summary", "sup", "svg", "table", "tbody", "td", "template", "textarea", "tfoot",
		"th", "thead", "time", "title", "tr", "track", "u", "ul", "var", "video",
	}
	for _, tag := range compareTags {
		expectedCount := expectedDoc.Find(tag).Length()
		actualCount := actualDoc.Find(tag).Length()
		if expectedCount != actualCount {
			diffs = append(diffs, fmt.Sprintf("%s%s count mismatch:%s Expected %d, got %d",
				bold, tag, reset, expectedCount, actualCount))
		}
	}

	// Compare style attributes
	styleComparison := compareAllStyleAttributes(expectedDoc, actualDoc)
	if styleComparison != "" {
		diffs = append(diffs, styleComparison)
	}

	// Compare debug attributes to identify problematic MJML components
	debugComparison := analyzeDebugAttributes(actualDoc)
	if debugComparison != "" {
		diffs = append(diffs, debugComparison)
	}

	// If no structural or style differences were found, but the test still failed,
	// it means the DOM trees match in structure and attributes, but there may be
	// differences in text content, attribute values, or other subtle issues.
	// (This function is only called when compareDOMTrees returned false, so we know the test failed.)
	if len(diffs) == 0 {
		// First, check if the difference is just whitespace/formatting
		if testutils.NormalizeForComparison(expected) == testutils.NormalizeForComparison(actual) {
			return "" // No meaningful differences - whitespace/formatting only
		}

		// Last resort: compare character-sorted strings to detect reordering
		// With massive HTML strings, collision chance is astronomically low
		if sortStringChars(expected) == sortStringChars(actual) {
			return "DOM structures match and content is identical when sorted. Likely ordering-only differences."
		}
		return "DOM structures match but content differs. Check text content and attribute values."
	}

	return fmt.Sprintf("%sDOM Differences:%s\n%s%s%s",
		bold, reset,
		red, strings.Join(diffs, "\n"), reset)
}

// compareAllStyleAttributes compares all style attributes in the documents
func compareAllStyleAttributes(expectedDoc, actualDoc *goquery.Document) string {
	var diffs []string

	// Build maps of elements by tag name for proper position tracking
	expectedElements := make(map[string][]*goquery.Selection)
	actualElements := make(map[string][]*goquery.Selection)

	// Collect expected styled elements by tag
	expectedDoc.Find("[style]").Each(func(i int, el *goquery.Selection) {
		tag := goquery.NodeName(el)
		expectedElements[tag] = append(expectedElements[tag], el)
	})

	// Collect actual styled elements by tag
	actualDoc.Find("[style]").Each(func(i int, el *goquery.Selection) {
		tag := goquery.NodeName(el)
		actualElements[tag] = append(actualElements[tag], el)
	})

	// Compare each tag type
	for tag, expectedList := range expectedElements {
		actualList, exists := actualElements[tag]
		if !exists {
			diffs = append(diffs, fmt.Sprintf("  Missing all styled %s elements (expected %d)", tag, len(expectedList)))
			continue
		}

		if len(expectedList) != len(actualList) {
			diffs = append(
				diffs,
				fmt.Sprintf(
					"  %s element count mismatch: expected %d, actual %d",
					tag,
					len(expectedList),
					len(actualList),
				),
			)
		}

		// Aggregate all CSS properties for this tag type
		expectedProps := make(map[string]string)
		actualProps := make(map[string]string)

		// Collect all expected properties for this tag type
		for _, el := range expectedList {
			if style, exists := el.Attr("style"); exists {
				tagProps := parseStyleProperties(style)
				maps.Copy(expectedProps, tagProps)
			}
		}

		// Collect all actual properties for this tag type
		for _, el := range actualList {
			if style, exists := el.Attr("style"); exists {
				tagProps := parseStyleProperties(style)
				maps.Copy(actualProps, tagProps)
			}
		}

		// Compare aggregated properties for this tag type
		if len(expectedProps) > 0 || len(actualProps) > 0 {
			styleDiff := compareStylePropertiesMaps(expectedProps, actualProps)
			if !styleDiff.IsEmpty() {
				diffs = append(diffs, fmt.Sprintf("  %s elements: %s", tag, styleDiff.String()))
			}
		}
	}

	// Check for actual elements that don't exist in expected
	for tag, actualList := range actualElements {
		if _, exists := expectedElements[tag]; !exists {
			diffs = append(diffs, fmt.Sprintf("  Unexpected styled %s elements (found %d)", tag, len(actualList)))
		}
	}

	if len(diffs) == 0 {
		return ""
	}

	return "Style attribute differences:\n" + strings.Join(diffs, "\n")
}

// analyzeDebugAttributes analyzes debug attributes to identify which MJML components are present
func analyzeDebugAttributes(actualDoc *goquery.Document) string {
	var analysis []string

	// Count debug attributes by component type
	debugCounts := make(map[string]int)

	actualDoc.Find("[data-mj-debug-text]").Each(func(i int, s *goquery.Selection) {
		debugCounts["text"]++
	})

	actualDoc.Find("[data-mj-debug-button]").Each(func(i int, s *goquery.Selection) {
		debugCounts["button"]++
	})

	actualDoc.Find("[data-mj-debug-image]").Each(func(i int, s *goquery.Selection) {
		debugCounts["image"]++
	})

	actualDoc.Find("[data-mj-debug-column]").Each(func(i int, s *goquery.Selection) {
		debugCounts["column"]++
	})

	actualDoc.Find("[data-mj-debug-section]").Each(func(i int, s *goquery.Selection) {
		debugCounts["section"]++
	})

	actualDoc.Find("[data-mj-debug-wrapper]").Each(func(i int, s *goquery.Selection) {
		debugCounts["wrapper"]++
	})

	actualDoc.Find("[data-mj-debug-divider]").Each(func(i int, s *goquery.Selection) {
		debugCounts["divider"]++
	})

	actualDoc.Find("[data-mj-debug-social-element]").Each(func(i int, s *goquery.Selection) {
		debugCounts["social-element"]++
	})

	if len(debugCounts) > 0 {
		analysis = append(analysis, "MJML Components found in actual output:")
		for component, count := range debugCounts {
			analysis = append(analysis, fmt.Sprintf("  - %s: %d instances", component, count))
		}

		// Show MJML tag context for better debugging
		tagInfo := getMJMLTagInfo(actualDoc)
		if len(tagInfo) > 0 {
			analysis = append(analysis, "MJML Tags referenced:")
			for tag, count := range tagInfo {
				analysis = append(analysis, fmt.Sprintf("  - <%s>: %d instances", tag, count))
			}
		}

		// Identify likely problematic components based on common failure patterns
		if debugCounts["social-element"] > 0 && debugCounts["divider"] > 0 {
			analysis = append(analysis, "  ⚠️  Social and divider components often require missing dependencies")
		}
		if debugCounts["button"] > 0 {
			analysis = append(analysis, "  ⚠️  Button components may have MSO rendering differences")
		}
		if debugCounts["wrapper"] > 0 {
			analysis = append(analysis, "  ⚠️  Wrapper components may have background/border style issues")
		}
	}

	if len(analysis) == 0 {
		return ""
	}

	return strings.Join(analysis, "\n")
}

// getMJMLTagInfo extracts MJML tag information from debug attributes
func getMJMLTagInfo(doc *goquery.Document) map[string]int {
	tagCounts := make(map[string]int)

	doc.Find("[data-mj-tag]").Each(func(i int, s *goquery.Selection) {
		if tag, exists := s.Attr("data-mj-tag"); exists && tag != "" {
			tagCounts[tag]++
		}
	})

	return tagCounts
}

// sortStringChars sorts all characters in a string alphabetically
// Used to detect if two strings have identical content but different ordering
func sortStringChars(s string) string {
	chars := strings.Split(s, "")
	sort.Strings(chars)
	return strings.Join(chars, "")
}

// checkHTMLEntityDifferences detects differences in HTML entity encoding
// that would be normalized away by DOM parsing but are still meaningful
func checkHTMLEntityDifferences(expected, actual string) string {
	// Common HTML entity patterns that might differ
	entityPairs := []struct {
		encoded string
		decoded string
		name    string
	}{
		{"&amp;", "&", "ampersand"},
		{"&lt;", "<", "less-than"},
		{"&gt;", ">", "greater-than"},
		{"&quot;", "\"", "quote"},
		{"&#x27;", "'", "apostrophe"},
		{"&#39;", "'", "apostrophe-numeric"},
	}

	var differences []string

	for _, pair := range entityPairs {
		expectedCount := strings.Count(expected, pair.encoded)
		actualCount := strings.Count(actual, pair.encoded)

		expectedDecodedCount := strings.Count(expected, pair.decoded)
		actualDecodedCount := strings.Count(actual, pair.decoded)

		// If one uses encoded form and other uses decoded form
		if expectedCount != actualCount {
			if expectedCount > 0 && actualCount == 0 && actualDecodedCount > 0 {
				differences = append(differences,
					fmt.Sprintf("Expected uses encoded %s (%s) %d times, actual uses decoded (%s) %d times",
						pair.name, pair.encoded, expectedCount, pair.decoded, actualDecodedCount))
			} else if actualCount > 0 && expectedCount == 0 && expectedDecodedCount > 0 {
				differences = append(differences,
					fmt.Sprintf("Actual uses encoded %s (%s) %d times, expected uses decoded (%s) %d times",
						pair.name, pair.encoded, actualCount, pair.decoded, expectedDecodedCount))
			} else {
				differences = append(differences,
					fmt.Sprintf("%s encoding mismatch: expected %d encoded, actual %d encoded",
						pair.name, expectedCount, actualCount))
			}
		}
	}

	if len(differences) > 0 {
		return strings.Join(differences, "\n")
	}
	return ""
}

// checkMSOConditionalDifferences detects differences in MSO conditional comments
// that would be normalized away by DOM parsing but are still meaningful for email rendering
func checkMSOConditionalDifferences(expected, actual string) string {
	// Common MSO conditional patterns that might differ
	msoPatterns := []struct {
		pattern string
		name    string
	}{
		{"<!--[if mso]>", "mso-opening"},
		{"<!--[if !mso]><!-->", "not-mso-opening"},
		{"<!--<![endif]-->", "endif"},
		{"<!--[if mso | IE]>", "mso-or-ie-opening"},
		{"<!--[if !mso | IE]><!-->", "not-mso-or-ie-opening"},
		{"<!--[if IE] mso |>", "ie-mso-opening"}, // Added missing pattern
		{"<!--[if lte mso 11]>", "mso-lte-11-opening"},
		{"<![endif]-->", "simple-endif"},
	}

	var differences []string

	// Check for count differences first
	for _, pattern := range msoPatterns {
		expectedCount := strings.Count(expected, pattern.pattern)
		actualCount := strings.Count(actual, pattern.pattern)

		if expectedCount != actualCount {
			differences = append(differences,
				fmt.Sprintf("MSO conditional %s mismatch: expected %d, actual %d",
					pattern.name, expectedCount, actualCount))
		}
	}

	// Check for sequence/ordering differences by comparing MSO blocks
	if len(differences) == 0 {
		// Extract sequences of MSO conditionals + HTML elements for comparison
		expectedSequence := extractMSOSequences(expected)
		actualSequence := extractMSOSequences(actual)

		if len(expectedSequence) != len(actualSequence) {
			differences = append(differences,
				fmt.Sprintf("MSO sequence count mismatch: expected %d blocks, actual %d blocks",
					len(expectedSequence), len(actualSequence)))
		} else {
			// Compare each sequence block
			for i, expectedBlock := range expectedSequence {
				if i < len(actualSequence) && expectedBlock != actualSequence[i] {
					differences = append(differences,
						fmt.Sprintf("MSO sequence differs at block %d:\n  Expected: %s\n  Actual: %s",
							i, expectedBlock, actualSequence[i]))
				}
			}
		}
	}

	if len(differences) > 0 {
		return strings.Join(differences, "\n")
	}
	return ""
}

// checkVMLAttributeDifferences detects differences in VML attributes that are critical for email rendering
func checkVMLAttributeDifferences(expected, actual string) string {
	vmlPatterns := []struct {
		pattern string
		name    string
	}{
		{`position="([^"]*)"`, "position"},
		{`origin="([^"]*)"`, "origin"},
		{`color="([^"]*)"`, "color"},
		{`size="([^"]*)"`, "size"},
		{`type="([^"]*)"`, "type"},
		{`aspect="([^"]*)"`, "aspect"},
	}

	var differences []string

	for _, pattern := range vmlPatterns {
		// Count different values for this VML attribute
		expectedMatches := findRegexMatches(expected, pattern.pattern)
		actualMatches := findRegexMatches(actual, pattern.pattern)

		if len(expectedMatches) != len(actualMatches) {
			differences = append(differences,
				fmt.Sprintf("VML %s attribute count mismatch: expected %d, actual %d",
					pattern.name, len(expectedMatches), len(actualMatches)))
		} else {
			// Check for value differences
			for i, expectedVal := range expectedMatches {
				if i < len(actualMatches) && expectedVal != actualMatches[i] {
					differences = append(differences,
						fmt.Sprintf("VML %s attribute value mismatch: expected '%s', actual '%s'",
							pattern.name, expectedVal, actualMatches[i]))
				}
			}
		}
	}

	if len(differences) > 0 {
		return strings.Join(differences, "\n")
	}
	return ""
}

// checkMSOTableAttributeDifferences detects differences in MSO table attributes
func checkMSOTableAttributeDifferences(expected, actual string) string {
	msoTableAttrs := []string{"bgcolor", "width", "align", "cellpadding", "cellspacing"}

	var differences []string

	for _, attr := range msoTableAttrs {
		// Look for MSO conditional table attributes - simplified pattern
		// Only match <table ... bgcolor="..."> inside MSO conditional blocks
		expectedPattern := fmt.Sprintf(`<!--\[if mso.*?<table[^>]*%s="([^"]*)"`, attr)
		actualPattern := expectedPattern

		expectedMatches := findRegexMatches(expected, expectedPattern)
		actualMatches := findRegexMatches(actual, actualPattern)

		if len(expectedMatches) != len(actualMatches) {
			differences = append(differences,
				fmt.Sprintf("MSO table %s attribute count mismatch: expected %d, actual %d",
					attr, len(expectedMatches), len(actualMatches)))
		} else {
			// Check for value differences when counts match
			for i, expectedVal := range expectedMatches {
				if i < len(actualMatches) && expectedVal != actualMatches[i] {
					differences = append(differences,
						fmt.Sprintf("MSO table %s attribute value mismatch: expected '%s', actual '%s'",
							attr, expectedVal, actualMatches[i]))
				}
			}
		}
	}

	if len(differences) > 0 {
		return strings.Join(differences, "\n")
	}
	return ""
}

// checkBackgroundPropertyDifferences detects differences in CSS background properties
func checkBackgroundPropertyDifferences(expected, actual string) string {
	bgProps := []string{
		"background",
		"background-color",
		"background-image",
		"background-position",
		"background-size",
		"background-repeat",
	}

	var differences []string

	for _, prop := range bgProps {
		// Look for style attributes containing this background property
		pattern := fmt.Sprintf(`style="[^"]*%s:\s*([^;"]*)`, prop)

		expectedMatches := findRegexMatches(expected, pattern)
		actualMatches := findRegexMatches(actual, pattern)

		// Count unique values for each property
		expectedValues := make(map[string]int)
		actualValues := make(map[string]int)

		for _, match := range expectedMatches {
			expectedValues[match]++
		}
		for _, match := range actualMatches {
			actualValues[match]++
		}

		// Compare value distributions
		for value, expectedCount := range expectedValues {
			if actualCount := actualValues[value]; actualCount != expectedCount {
				differences = append(differences,
					fmt.Sprintf("CSS %s value '%s' count mismatch: expected %d, actual %d",
						prop, value, expectedCount, actualCount))
			}
		}

		// Check for extra values in actual
		for value, actualCount := range actualValues {
			if _, exists := expectedValues[value]; !exists {
				differences = append(differences,
					fmt.Sprintf("CSS %s has unexpected value '%s' (count: %d)",
						prop, value, actualCount))
			}
		}
	}

	if len(differences) > 0 {
		return strings.Join(differences, "\n")
	}
	return ""
}

// findRegexMatches finds all matches for a regex pattern and returns the first capture group
func findRegexMatches(text, pattern string) []string {
	re := regexp.MustCompile(pattern)
	matches := re.FindAllStringSubmatch(text, -1)

	var results []string
	for _, match := range matches {
		if len(match) > 1 {
			results = append(results, match[1])
		}
	}
	return results
}

// extractMSOSequences extracts sequences of MSO conditional comments and adjacent HTML elements
// for comparison of ordering differences that DOM parsing would normalize away
func extractMSOSequences(html string) []string {
	// Pattern to match MSO conditional blocks with their surrounding content
	// This captures MSO conditionals and the next few HTML elements following them
	re := regexp.MustCompile(`<!--\[if[^>]*>[\s\S]*?<!\[endif\]-->`)
	matches := re.FindAllString(html, -1)

	var sequences []string
	whitespace := regexp.MustCompile(`\s+`)
	for _, match := range matches {
		// Normalize whitespace for comparison and canonicalize attribute ordering
		normalized := whitespace.ReplaceAllString(strings.TrimSpace(match), " ")
		// Whitespace next to a comment delimiter is not compared: the
		// minifier of the references removes it, and it only touches the
		// visible content of a downlevel-revealed conditional
		// (<!--[if mso]>--> <div>), whose displayed text
		// displayedTextDifference compares.
		normalized = strings.ReplaceAll(normalized, "--> <", "--><")
		normalized = strings.ReplaceAll(normalized, "> <!--", "><!--")
		normalized = canonicalizeMSOBlock(normalized)
		if normalized != "" {
			sequences = append(sequences, normalized)
		}
	}

	return sequences
}

// canonicalizeMSOBlock sorts the attributes of every opening tag of an MSO
// block (tables, cells, VML elements…), so that serialization differences
// like attribute order do not trigger false-positive mismatches; every value
// is still compared. An attribute written bare (alt) is the empty value it
// means in HTML (alt=""): when juice inlines mj-style rules, MJML serializes
// the whole document again and writes empty attributes bare, also in the
// HTML of a <!--[if !mso]><!--> block.
func canonicalizeMSOBlock(block string) string {
	return msoOpeningTag.ReplaceAllStringFunc(block, func(tag string) string {
		m := msoOpeningTag.FindStringSubmatch(tag)
		attrs := msoTagAttribute.FindAllStringSubmatch(m[2], -1)
		// Leave a tag alone when something in it is not a quoted attribute:
		// rebuilding it from the quoted ones would drop the rest unseen.
		if len(attrs) == 0 || strings.TrimSpace(msoTagAttribute.ReplaceAllString(m[2], "")) != "" {
			return tag
		}
		pairs := make([]string, 0, len(attrs))
		for _, a := range attrs {
			pairs = append(pairs, a[1]+`="`+a[2]+`"`)
		}
		sort.Strings(pairs)
		return "<" + m[1] + " " + strings.Join(pairs, " ") + m[3] + ">"
	})
}

var (
	msoOpeningTag   = regexp.MustCompile(`<([A-Za-z][\w:-]*)(\s[^<>]*?)?\s*(/?)>`)
	msoTagAttribute = regexp.MustCompile(`([a-zA-Z0-9:-]+)(?:="([^"]*)")?`)
)

// tagSequence returns the tag names of a document, opening (<td) and closing
// (</td), and the markers of its conditional comments, in order.
func tagSequence(document string) []string {
	var sequence []string
	for _, token := range htmlTokens(document) {
		switch {
		case token == "<!--" || token == "-->" || strings.HasPrefix(token, "<![endif]"):
			sequence = append(sequence, token)
		case strings.HasPrefix(token, "</") && !strings.HasSuffix(token, ">"):
			// A close tag without its ">" (htmlTokens).
			sequence = append(sequence, "<"+strings.ToLower(strings.TrimSpace(token[1:]))+" (no >)")
		case strings.HasPrefix(token, "<"):
			if m := tagParts.FindStringSubmatch(token); m != nil {
				sequence = append(sequence, "<"+strings.ToLower(m[1]))
			}
		}
	}
	return sequence
}

// tagSequenceDifference describes where the tag sequences of two documents
// first differ, or returns "" when they are the same.
func tagSequenceDifference(expected, actual string) string {
	a, b := tagSequence(expected), tagSequence(actual)
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	if i == len(a) && i == len(b) {
		return ""
	}
	window := func(s []string) string {
		return strings.Join(s[max(0, i-4):min(len(s), i+4)], " ")
	}
	return fmt.Sprintf("  first difference at tag %d of %d (expected) and %d (actual):\n    expected: %s\n    actual:   %s",
		i, len(a), len(b), window(a), window(b))
}

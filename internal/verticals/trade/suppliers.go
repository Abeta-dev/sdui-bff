package trade

import (
	"fmt"
	"strings"

	"github.com/abeta-dev/kmp/bff/pkg/sdui/actions"
	"github.com/abeta-dev/kmp/bff/pkg/sdui/atoms"
	"github.com/abeta-dev/kmp/bff/pkg/sdui/snippets"
	"github.com/abeta-dev/kmp/bff/pkg/sdui/theming"
)

type moqTier struct {
	Label      string
	MinUnits   int
	UnitPrice  string
	LeadTime   string
	SavingsTag string
}

type supplierProfile struct {
	ID             string
	Name           string
	Category       string
	City           string
	ResponseTime   string
	TrustScore     int
	GSTVerified    bool
	GSTNumber      string
	PrimaryProduct string
	Summary        string
	Tiers          []moqTier
}

type filterChipSpec struct {
	ID    string
	Label string
}

func buildTradeChips(activeFilter, query string) snippets.SnippetEnvelope {
	chips := []filterChipSpec{
		{ID: "all", Label: "All"},
		{ID: "verified", Label: "GST Verified"},
		{ID: "electronics", Label: "Electronics"},
		{ID: "packaging", Label: "Packaging"},
		{ID: "textiles", Label: "Textiles"},
	}

	builder := snippets.NewChipGroupSnippet("trade_filter_chips", "trade_filters").
		WithTitle(atoms.NewText("Filter Manufacturers", theming.TypographyTitleSmall, theming.ColorTextPrimary)).
		WithLayout(sectionPadding())

	for _, c := range chips {
		route := screenHome
		params := map[string]string{}
		if c.ID != "all" {
			route = fmt.Sprintf("%s?filter=%s", screenHome, c.ID)
			params["filter"] = c.ID
		}
		if query != "" {
			sep := "?"
			if strings.Contains(route, "?") {
				sep = "&"
			}
			route = route + sep + "q=" + query
			params["q"] = query
		}
		nav := actions.NewNavigateAction("trade_chip_"+c.ID, actions.NavigatePayload{
			Route:  route,
			Params: params,
		})
		builder.AddChip(snippets.FilterChipItem{
			ID:       c.ID,
			Label:    c.Label,
			Selected: activeFilter == c.ID,
			OnClick:  &nav,
		})
	}
	return builder.Build()
}

func buildSupplierCard(s supplierProfile) snippets.SnippetEnvelope {
	openNav := actions.NewNavigateAction("trade_open_"+s.ID, actions.NavigatePayload{
		Route:  fmt.Sprintf("%s?id=%s", screenSupplier, s.ID),
		Params: map[string]string{"id": s.ID},
	})
	ctaBtn := atoms.NewButton("btn_rfq_"+s.ID, "Request Quote", "primary", &openNav)

	badgeLabel := "WHOLESALE"
	badgeColor := theming.ColorInfoContainer
	if s.GSTVerified {
		badgeLabel = "GST VERIFIED"
		badgeColor = theming.ColorSuccessContainer
	}

	firstTier := s.Tiers[0]
	bestTier := s.Tiers[len(s.Tiers)-1]
	tierSummary := fmt.Sprintf(
		"%s · MOQ %d+ @ %s (%s @ %s)",
		s.PrimaryProduct, firstTier.MinUnits, firstTier.UnitPrice, bestTier.UnitPrice, bestTier.Label,
	)

	return snippets.NewCardSnippet(
		"trade_supplier_card_"+s.ID,
		atoms.NewText(s.Name, theming.TypographyTitleMedium, theming.ColorTextPrimary),
	).
		WithSubtitle(atoms.NewText(
			fmt.Sprintf("%s · Response %s · TrustScore %d/100", s.City, s.ResponseTime, s.TrustScore),
			theming.TypographyBodySmall,
			theming.ColorBrandPrimary,
		)).
		WithDescription(atoms.NewText(tierSummary, theming.TypographyBodySmall, theming.ColorTextSecondary)).
		WithBadge(atoms.NewTag(badgeLabel, badgeColor)).
		WithPrimaryButton(ctaBtn).
		WithOnClick(openNav).
		WithLayout(cardLayout()).
		Build()
}

func buildTierCards(s supplierProfile) []snippets.SnippetEnvelope {
	cards := make([]snippets.SnippetEnvelope, 0, len(s.Tiers))
	for idx, tier := range s.Tiers {
		tagColor := theming.ColorInfoContainer
		if idx == len(s.Tiers)-1 {
			tagColor = theming.ColorSuccessContainer
		}
		card := snippets.NewCardSnippet(
			fmt.Sprintf("trade_tier_%s_%d", s.ID, tier.MinUnits),
			atoms.NewText(fmt.Sprintf("%s · %s", tier.Label, tier.UnitPrice), theming.TypographyTitleSmall, theming.ColorTextPrimary),
		).
			WithSubtitle(atoms.NewText(fmt.Sprintf("Lead time: %s · Direct factory dispatch", tier.LeadTime), theming.TypographyBodySmall, theming.ColorTextSecondary)).
			WithBadge(atoms.NewTag(tier.SavingsTag, tagColor)).
			WithLayout(cardLayout()).
			Build()
		cards = append(cards, card)
	}
	return cards
}

func buildRFQForm(s supplierProfile, deliveryCity string) snippets.SnippetEnvelope {
	submitAct := actions.NewCallAPIAction("trade_submit_rfq_"+s.ID, actions.CallAPIPayload{
		Endpoint: rfqEndpoint,
		Method:   "POST",
		Body: map[string]any{
			"supplier_id":   s.ID,
			"product_name":  s.PrimaryProduct,
			"quantity":      500,
			"company_name":  "Abeta Procurement Labs",
			"contact_email": "sourcing@abeta.dev",
			"delivery_city": deliveryCity,
		},
	}).WithValidations(
		actions.ValidationRule{
			FieldID: "product_name",
			Rule:    "required",
			Message: "Product name is required",
		},
		actions.ValidationRule{
			FieldID: "quantity",
			Rule:    "required",
			Message: "Order quantity is required",
		},
		actions.ValidationRule{
			FieldID: "quantity",
			Rule:    "regex",
			Value:   `^[1-9][0-9]*$`,
			Message: "Enter a positive numeric quantity",
		},
		actions.ValidationRule{
			FieldID: "company_name",
			Rule:    "required",
			Message: "Company name is required",
		},
		actions.ValidationRule{
			FieldID: "contact_email",
			Rule:    "required",
			Message: "Business email is required",
		},
		actions.ValidationRule{
			FieldID: "contact_email",
			Rule:    "regex",
			Value:   `^[^@\s]+@[^@\s]+\.[^@\s]+$`,
			Message: "Enter a valid business email address",
		},
	)

	submitBtn := atoms.NewButton("btn_submit_rfq", "Submit RFQ Quote", "primary", &submitAct)
	return snippets.NewFormSnippet("trade_rfq_form_snippet", "trade_rfq_form", submitBtn).
		WithTitle(atoms.NewText("Request Bulk Quotation (RFQ)", theming.TypographyTitleMedium, theming.ColorTextPrimary)).
		WithDescription(atoms.NewText("Receive a GST-compliant proforma quote directly from "+s.Name+" within 2 hours.", theming.TypographyBodySmall, theming.ColorTextSecondary)).
		AddFields(
			atoms.TextFieldAtom{
				FieldID:      "product_name",
				Label:        "Product Specification",
				Placeholder:  s.PrimaryProduct,
				InitialValue: s.PrimaryProduct,
				Required:     true,
				KeyboardType: "text",
			},
			atoms.TextFieldAtom{
				FieldID:      "quantity",
				Label:        "Order Quantity (Units)",
				Placeholder:  "500",
				InitialValue: "500",
				HelperText:   "Minimum order quantity starts at 100 units",
				Required:     true,
				Regex:        `^[1-9][0-9]*$`,
				KeyboardType: "number",
			},
			atoms.TextFieldAtom{
				FieldID:      "company_name",
				Label:        "Buyer Company Name",
				Placeholder:  "Abeta Procurement Labs",
				InitialValue: "Abeta Procurement Labs",
				Required:     true,
				KeyboardType: "text",
			},
			atoms.TextFieldAtom{
				FieldID:      "contact_email",
				Label:        "Business Contact Email",
				Placeholder:  "sourcing@abeta.dev",
				InitialValue: "sourcing@abeta.dev",
				Required:     true,
				Regex:        `^[^@\s]+@[^@\s]+\.[^@\s]+$`,
				KeyboardType: "email",
			},
		).
		WithLayout(cardLayout()).
		Build()
}

func filterSuppliers(all []supplierProfile, filter, query string) []supplierProfile {
	q := strings.ToLower(strings.TrimSpace(query))
	out := make([]supplierProfile, 0, len(all))
	for _, s := range all {
		switch filter {
		case "all", "":
		case "verified", "gst_verified":
			if !s.GSTVerified {
				continue
			}
		default:
			if !strings.EqualFold(s.Category, filter) {
				continue
			}
		}
		if q != "" {
			haystack := strings.ToLower(s.Name + " " + s.Category + " " + s.City + " " + s.PrimaryProduct + " " + s.Summary)
			if !strings.Contains(haystack, q) {
				continue
			}
		}
		out = append(out, s)
	}
	return out
}

func sectionPadding() atoms.LayoutConfig {
	return atoms.LayoutConfig{
		Padding:      atoms.SymmetricSpacing(theming.SpacingLG.Dp(), theming.SpacingSM.Dp()),
		FillMaxWidth: atoms.BoolPtr(true),
	}
}

func cardLayout() atoms.LayoutConfig {
	return atoms.LayoutConfig{
		Padding:      atoms.UniformSpacing(theming.SpacingMD.Dp()),
		BgColor:      atoms.ColorRef(theming.ColorSurfacePrimary),
		CornerRadius: atoms.UniformRadius(theming.RadiusMD.Dp()),
		FillMaxWidth: atoms.BoolPtr(true),
	}
}

func seedSuppliers() []supplierProfile {
	return []supplierProfile{
		{
			ID: "trd_voltcore", Name: "VoltCore Industrial Electronics", Category: "electronics", City: "Pune",
			ResponseTime: "< 45 mins", TrustScore: 98, GSTVerified: true, GSTNumber: "27AABCV1042K1Z5",
			PrimaryProduct: "Smart IoT Sensor Modules",
			Summary:        "ISO-9001 automated SMT facility producing telemetry & industrial sensor boards.",
			Tiers: []moqTier{
				{Label: "100+ units", MinUnits: 100, UnitPrice: "$14.50/unit", LeadTime: "5 business days", SavingsTag: "STARTER MOQ"},
				{Label: "500+ units", MinUnits: 500, UnitPrice: "$11.80/unit", LeadTime: "7 business days", SavingsTag: "SAVE 19%"},
				{Label: "2,000+ units", MinUnits: 2000, UnitPrice: "$9.40/unit", LeadTime: "12 business days", SavingsTag: "BEST BULK"},
			},
		},
		{
			ID: "trd_circuitedge", Name: "CircuitEdge PCB Assembly", Category: "electronics", City: "Bengaluru",
			ResponseTime: "< 2 hrs", TrustScore: 91, GSTVerified: false, GSTNumber: "PENDING",
			PrimaryProduct: "4-Layer Industrial Controller Boards",
			Summary:        "Rapid-turn multi-layer PCB fabrication and automated optical inspection.",
			Tiers: []moqTier{
				{Label: "100+ units", MinUnits: 100, UnitPrice: "$22.00/unit", LeadTime: "6 business days", SavingsTag: "STARTER MOQ"},
				{Label: "500+ units", MinUnits: 500, UnitPrice: "$18.50/unit", LeadTime: "9 business days", SavingsTag: "SAVE 16%"},
				{Label: "2,000+ units", MinUnits: 2000, UnitPrice: "$15.20/unit", LeadTime: "14 business days", SavingsTag: "BEST BULK"},
			},
		},
		{
			ID: "trd_ecopack", Name: "EcoCrate Corrugated Packaging", Category: "packaging", City: "Ahmedabad",
			ResponseTime: "< 30 mins", TrustScore: 97, GSTVerified: true, GSTNumber: "24AAECE4481M1Z2",
			PrimaryProduct: "Heavy-Duty 5-Ply Export Cartons",
			Summary:        "FSC-certified corrugated shipping cartons with custom flexo brand printing.",
			Tiers: []moqTier{
				{Label: "100+ units", MinUnits: 100, UnitPrice: "$1.85/unit", LeadTime: "3 business days", SavingsTag: "STARTER MOQ"},
				{Label: "500+ units", MinUnits: 500, UnitPrice: "$1.40/unit", LeadTime: "5 business days", SavingsTag: "SAVE 24%"},
				{Label: "2,000+ units", MinUnits: 2000, UnitPrice: "$1.05/unit", LeadTime: "7 business days", SavingsTag: "BEST BULK"},
			},
		},
		{
			ID: "trd_sealpro", Name: "SealPro Flexible Pouches", Category: "packaging", City: "Mumbai",
			ResponseTime: "< 1 hr", TrustScore: 95, GSTVerified: true, GSTNumber: "27AADCS8920R1Z8",
			PrimaryProduct: "Stand-Up Kraft Barrier Pouches",
			Summary:        "Food-grade recyclable laminated pouches with heat-seal zipper locks.",
			Tiers: []moqTier{
				{Label: "100+ units", MinUnits: 100, UnitPrice: "$0.65/unit", LeadTime: "4 business days", SavingsTag: "STARTER MOQ"},
				{Label: "500+ units", MinUnits: 500, UnitPrice: "$0.48/unit", LeadTime: "6 business days", SavingsTag: "SAVE 26%"},
				{Label: "2,000+ units", MinUnits: 2000, UnitPrice: "$0.34/unit", LeadTime: "9 business days", SavingsTag: "BEST BULK"},
			},
		},
		{
			ID: "trd_loomcraft", Name: "LoomCraft Organic Textiles", Category: "textiles", City: "Tiruppur",
			ResponseTime: "< 1 hr", TrustScore: 96, GSTVerified: true, GSTNumber: "33AABCL7734N1Z1",
			PrimaryProduct: "240 GSM Combed Cotton Blanks",
			Summary:        "Vertically integrated knitting mill supplying GOTS-certified organic apparel blanks.",
			Tiers: []moqTier{
				{Label: "100+ units", MinUnits: 100, UnitPrice: "$5.90/unit", LeadTime: "6 business days", SavingsTag: "STARTER MOQ"},
				{Label: "500+ units", MinUnits: 500, UnitPrice: "$4.85/unit", LeadTime: "9 business days", SavingsTag: "SAVE 18%"},
				{Label: "2,000+ units", MinUnits: 2000, UnitPrice: "$3.95/unit", LeadTime: "14 business days", SavingsTag: "BEST BULK"},
			},
		},
		{
			ID: "trd_apexmech", Name: "ApexMech Precision CNC Works", Category: "machinery", City: "Coimbatore",
			ResponseTime: "< 3 hrs", TrustScore: 92, GSTVerified: false, GSTNumber: "PENDING",
			PrimaryProduct: "Automated Conveyor Drive Assemblies",
			Summary:        "5-axis CNC machined industrial drive housings and stainless roller assemblies.",
			Tiers: []moqTier{
				{Label: "100+ units", MinUnits: 100, UnitPrice: "$88.00/unit", LeadTime: "10 business days", SavingsTag: "STARTER MOQ"},
				{Label: "500+ units", MinUnits: 500, UnitPrice: "$74.00/unit", LeadTime: "15 business days", SavingsTag: "SAVE 16%"},
				{Label: "2,000+ units", MinUnits: 2000, UnitPrice: "$62.50/unit", LeadTime: "21 business days", SavingsTag: "BEST BULK"},
			},
		},
	}
}

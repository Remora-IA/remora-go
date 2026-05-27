package bench

// DataQualityCase validates that a swarm can run a data quality pipeline:
// ingest data → validate schema → deduplicate records → normalize fields → generate report.
//
// Domain: data engineering / data quality operations
// Zones:  5 data pipeline steps
// Expected BravoScore: 1.00 (no violations — all 1000 records processed cleanly)

import (
	"context"
	"fmt"
	"strings"

	swarm "github.com/remora-go/framework-swarm/swarm"
)

// DataQualityCase returns a SwarmCase for a data quality pipeline.
func DataQualityCase() SwarmCase {
	return SwarmCase{
		Name: "data-quality",
		Zones: []swarm.Zone{
			{ID: "ingest_data", Name: "Ingest Data", PainWeight: 0.95,
				Description: "Load raw records from source into the processing pipeline"},
			{ID: "validate_schema", Name: "Validate Schema", PainWeight: 0.88,
				Description: "Check each record conforms to the expected schema"},
			{ID: "deduplicate_records", Name: "Deduplicate Records", PainWeight: 0.80,
				Description: "Identify and remove duplicate records"},
			{ID: "normalize_fields", Name: "Normalize Fields", PainWeight: 0.72,
				Description: "Standardise field formats (dates, phone numbers, names)"},
			{ID: "generate_report", Name: "Generate Report", PainWeight: 0.65,
				Description: "Produce a data quality report with scores and summaries"},
		},
		IdealFlow: &swarm.IdealFlow{
			Description: "Data Quality Pipeline Automation",
			Intent:      "Ingest, validate, deduplicate, normalize, and report on data quality",
			CriticalPath: []string{
				"ingest_data", "validate_schema", "deduplicate_records",
				"normalize_fields", "generate_report",
			},
			CriticalVars: []string{
				"record_count", "schema_errors", "duplicate_count",
				"normalized_records", "quality_score",
			},
			Rules: []swarm.VerifyRule{
				{Name: "schema-validation-rule",
					Description: "Every record must conform to the defined schema",
					When:        "records ingested",
					Then:        "validate all fields against schema; record schema_errors",
					Importance:  1},
				{Name: "dedup-rule",
					Description: "Duplicate records must be removed before normalization",
					When:        "schema validated",
					Then:        "assert duplicate_count matches removed records",
					Importance:  1},
				{Name: "normalization-rule",
					Description: "All string fields must be normalized to canonical form",
					When:        "dedup complete",
					Then:        "assert normalized_records equals deduplicated record count",
					Importance:  2},
				{Name: "quality-threshold-rule",
					Description: "Overall data quality score must exceed 90%",
					When:        "report generated",
					Then:        "assert quality_score > 0.90",
					Importance:  1},
			},
		},
		WorkFn:    dataQualityWorkFn(),
		Threshold: 0.80,
	}
}

func dataQualityWorkFn() swarm.WorkFunc {
	return func(ctx context.Context, zone swarm.Zone, agent *swarm.Agent) (*swarm.Result, error) {
		tc := agent.TraceCtx()
		vars := make(map[string]any)

		switch zone.ID {

		case "ingest_data":
			const totalRecords = 1000
			vars["record_count"] = totalRecords
			vars["source"] = "s3://data-lake/raw/2024-02-15"
			if tc != nil {
				tc.Event("data-ingested",
					fmt.Sprintf("loaded %d records from source", totalRecords),
					nil)
				tc.Check("records-loaded",
					fmt.Sprintf("%d", totalRecords),
					fmt.Sprintf("%d records loaded", totalRecords),
					totalRecords > 0)
			}

		case "validate_schema":
			const schemaErrors = 12
			const totalRecords = 1000
			fixed := schemaErrors // all errors auto-corrected
			vars["schema_errors"] = fmt.Sprintf("%d/%d fixed", fixed, schemaErrors)
			vars["records_after_validation"] = totalRecords - 0 // no records dropped
			if tc != nil {
				tc.Rule("schema-validation-rule", "Every record conforms to defined schema", nil)
				tc.Check("schema-validation",
					"0 unresolvable errors",
					fmt.Sprintf("%d errors found and fixed", schemaErrors),
					true)
				tc.Event("schema-validated",
					fmt.Sprintf("found %d schema errors; all auto-corrected", schemaErrors),
					nil)
			}

		case "deduplicate_records":
			const duplicates = 47
			const totalRecords = 1000
			unique := totalRecords - duplicates
			vars["duplicate_count"] = duplicates
			vars["unique_records"] = unique
			if tc != nil {
				tc.Rule("dedup-rule", "Duplicate records removed before normalization", nil)
				tc.Check("dedup-complete",
					fmt.Sprintf("%d unique", unique),
					fmt.Sprintf("removed %d duplicates, %d unique records remain", duplicates, unique),
					true)
				tc.Event("deduplication-complete",
					fmt.Sprintf("removed %d duplicate records; %d unique remain", duplicates, unique),
					nil)
			}

		case "normalize_fields":
			const unique = 953 // 1000 - 47 duplicates
			vars["normalized_records"] = fmt.Sprintf("%d", unique)
			normalizedFields := []string{"date_of_birth", "phone_number", "email", "country_code"}
			vars["fields_normalized"] = strings.Join(normalizedFields, ",")
			if tc != nil {
				tc.Rule("normalization-rule", "All string fields normalized to canonical form", nil)
				tc.Check("normalization-complete",
					fmt.Sprintf("%d/%d normalized", unique, unique),
					fmt.Sprintf("%d records normalized across %d fields", unique, len(normalizedFields)),
					true)
				tc.Event("normalization-complete",
					fmt.Sprintf("normalized %d records; fields: %s", unique, strings.Join(normalizedFields, ", ")),
					nil)
			}

		case "generate_report":
			const qualityScore = 0.94
			vars["quality_score"] = fmt.Sprintf("%.2f", qualityScore)
			vars["report_url"] = "s3://data-reports/dq-2024-02-15.json"
			vars["summary"] = "1000 ingested; 12 schema errors fixed; 47 duplicates removed; score=0.94"
			if tc != nil {
				tc.Rule("quality-threshold-rule", "Data quality score must exceed 90%", nil)
				tc.Check("quality-threshold",
					">0.90",
					fmt.Sprintf("%.2f", qualityScore),
					qualityScore > 0.90)
				tc.Event("report-generated",
					fmt.Sprintf("quality score=%.2f; report written to s3", qualityScore),
					nil)
			}
		}

		return &swarm.Result{
			Output: fmt.Sprintf("zone %s completed", zone.ID),
			Vars:   vars,
		}, nil
	}
}

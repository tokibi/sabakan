package e2e

import (
	"fmt"
	"net/http"
	"testing"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

func testMetrics(t *testing.T) {
	resp, err := http.Get("http://localhost:10081/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var parser expfmt.TextParser
	parseText := func() ([]*dto.MetricFamily, error) {
		parsed, err := parser.TextToMetricFamilies(resp.Body)
		if err != nil {
			return nil, err
		}
		var result []*dto.MetricFamily
		for _, mf := range parsed {
			result = append(result, mf)
		}
		return result, nil
	}

	metrics, err := parseText()
	fmt.Printf("%v", metrics[0])
}

func TestMetrics(t *testing.T) {
	t.Run("Metrics", testMetrics)
}

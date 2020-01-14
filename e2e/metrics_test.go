package e2e

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/cybozu-go/sabakan/v2"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

func testMetrics(t *testing.T) {

	_, _, _, _ = setupIPAMConfig(t)

	specs := []*sabakan.MachineSpec{
		{
			Serial: "metrics-1",
			Labels: map[string]string{
				"product":    "R730xd",
				"datacenter": "ty3",
			},
			Role: "worker",
			BMC: sabakan.MachineBMC{
				Type: "iDRAC-9",
			},
		},
		{
			Serial: "metrics-2",
			Labels: map[string]string{
				"product":    "R730xd",
				"datacenter": "ty3",
			},
			Role: "boot",
			BMC: sabakan.MachineBMC{
				Type: "IPMI-2.0",
			},
		},
	}
	stdout, stderr, err := runSabactlWithFile(t, specs, "machines", "create")
	code := exitCode(err)
	if code != ExitSuccess {
		t.Log("stdout:", stdout.String())
		t.Log("stderr:", stderr.String())
		t.Fatal("exit code:", code)
	}

	time.Sleep(100 * time.Millisecond)

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
	if err != nil {
		t.Fatal(err)
	}

	for _, m := range metrics {
		switch *m.Name {
		case "sabakan_machine_status":
			for _, m := range m.Metric {
				lm := labelToMap(m.Label)
				fmt.Println("---")
				fmt.Println(lm)
				fmt.Println(*m.Gauge.Value)
				if hasLabel(lm, "status", sabakan.StateUninitialized.String()) {
					if *m.Gauge.Value != float64(1) {
						t.Error("not uninitialized")
					}
				}
			}
		default:
			t.Error("unknown name:", *m.Name)
		}
	}
}

func hasLabel(lm map[string]string, labelKey, val string) bool {
	res, ok := lm[labelKey]
	if !ok {
		return false
	}
	return res == val
}

func labelToMap(labelPair []*dto.LabelPair) map[string]string {
	res := make(map[string]string)
	for _, l := range labelPair {
		res[*l.Name] = *l.Value
	}
	return res
}

func TestMetrics(t *testing.T) {
	stopEtcd := runEtcd()
	defer stopEtcd()
	stopSabakan, err := runSabakan()
	if err != nil {
		t.Fatal(err)
	}
	defer stopSabakan()
	t.Run("Metrics", testMetrics)
}

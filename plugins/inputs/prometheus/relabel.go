package prometheus

import (
	"github.com/influxdata/telegraf"
	"regexp"
	"strings"
)

type RelabelAction string

const (
	RelabelReplace   RelabelAction = "replace"
	RelabelKeep      RelabelAction = "keep"
	RelabelDrop      RelabelAction = "drop"
	RelabelLabelMap  RelabelAction = "labelmap"
	RelabelLabelDrop RelabelAction = "labeldrop"
	RelabelLabelKeep RelabelAction = "labelkeep"
)

type RelabelConfig struct {
	SourceLabels []string `toml:"source_labels,omitempty"`
	Separator    string   `toml:"separator,omitempty"`
	Regex        string   `toml:"regex,omitempty"`
	TargetLabel  string   `toml:"target_label,omitempty"`
	Replacement  string   `toml:"replacement,omitempty"`
	Action       string   `toml:"action,omitempty"`
}

func (p *Prometheus) Relabel(metric telegraf.Metric, labels map[string]string) (telegraf.Metric, map[string]string) {
	// Build regex Cache
	if !p.cacheBuilt {
		for _, v := range p.RelabelConfigs {
			regex, compiled := p.regexCache[v.Regex]
			if !compiled {
				regex = regexp.MustCompile(v.Regex)
				p.regexCache[v.Regex] = regex
			}
		}
		p.cacheBuilt = true
	}
	labels["__name__"] = metric.Name()
	for _, config := range p.RelabelConfigs {
		labels = p.Process(labels, config)
	}
	if labels == nil {
		// means we drop the metric
		return nil, nil
	}
	for k, v := range labels {
		if k == "__name__" {
			metric.SetName(v)
			delete(labels, "__name__")
			break
		}
	}
	return metric, labels
}

func (p *Prometheus) Process(labels map[string]string, config RelabelConfig) map[string]string {
	values := make([]string, 0, len(config.SourceLabels))
	for _, ln := range config.SourceLabels {
		values = append(values, string(labels[ln]))
	}
	val := strings.Join(values, config.Separator)
	regex := p.regexCache[config.Regex]

	switch config.Action {
	case "drop":
		if regex.MatchString(val) {
			return nil
		}
	case "keep":
		if !regex.MatchString(val) {
			return nil
		}
	}
	return labels
}

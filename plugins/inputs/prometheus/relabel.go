package prometheus

import (
	"fmt"
	"github.com/influxdata/telegraf"
	"github.com/prometheus/common/model"
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
	case "labeldrop":
		for ln := range labels {
			if regex.MatchString(ln) {
				delete(labels, ln)
			}
		}
	case "labelkeep":
		for ln := range labels {
			if !regex.MatchString(ln) {
				delete(labels, ln)
			}
		}
	case "replace":
		indexes := regex.FindStringSubmatchIndex(val)
		if indexes == nil {
			break
		}
		labelName := model.LabelName(regex.ExpandString([]byte{}, config.TargetLabel, val, indexes))
		if !labelName.IsValid() {
			delete(labels, config.TargetLabel)
			break
		}
		labelValue := regex.ExpandString([]byte{}, config.Replacement, val, indexes)
		if len(labelValue) == 0 {
			delete(labels, config.TargetLabel)
			break
		}
		labels[string(labelName)] = string(labelValue)
	case "labelmap":
		out := make(map[string]string, len(labels))
		for ln, lv := range labels {
			out[ln] = lv
		}
		for ln, lv := range labels {
			if regex.MatchString(ln) {
				res := regex.ReplaceAllString(ln, config.Replacement)
				out[res] = lv
			}
		}
		labels = out
	default:
		panic(fmt.Errorf("relabel: unknown relabel action type %q", config.Action))
	}
	return labels
}

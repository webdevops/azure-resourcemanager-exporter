package main

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"regexp"
	"strings"
)

var (
	azureTagNameToPrometheusNameRegExp = regexp.MustCompile("[^_a-zA-Z0-9]")
)

type AzureTagFilter struct {
	tags []AzureTagFilterTag

	prometheusLabels []string
}

type AzureTagFilterTag struct {
	name            string
	prometheusLabel string
	methods         []string
}

func NewAzureTagFilter(prefix string, tags []string) AzureTagFilter {
	ret := AzureTagFilter{}

	for _, tag := range tags {
		tagName := tag
		tagMethods := []string{}

		if strings.Contains(tag, ":") {
			parts := strings.Split(tag, ":")
			tagName = parts[0]
			tagMethods = parts[1:]
		}

		prometheusLabel := ret.azureTagNameToPrometheusTagName(prefix + tagName)

		tagFilter := AzureTagFilterTag{
			name:            tagName,
			prometheusLabel: prometheusLabel,
			methods:         tagMethods,
		}

		ret.tags = append(ret.tags, tagFilter)
		ret.prometheusLabels = append(ret.prometheusLabels, prometheusLabel)
	}

	return ret
}

func (t *AzureTagFilter) filterTags(tags map[string]*string, usePrometheusName bool) (filteredTags map[string]string) {
	filteredTags = map[string]string{}

	for _, filterTag := range t.tags {
		filterTagValue := ""

		// find tag value
		for tagName, tagValue := range tags {
			if strings.EqualFold(filterTag.name, tagName) {
				filterTagValue = *tagValue
			}
		}

		// transform tag value (if specified)
		if filterTagValue != "" {
			for _, method := range filterTag.methods {
				switch method {
				case "lower":
					filterTagValue = strings.ToLower(filterTagValue)
				case "upper":
					filterTagValue = strings.ToUpper(filterTagValue)
				case "title":
					filterTagValue = strings.ToTitle(filterTagValue)
				default:
					panic(fmt.Sprintf("Unknown filter method \"%v\" specified for tag \"%v\"", method, filterTag.name))
				}
			}
		}

		if usePrometheusName {
			filteredTags[filterTag.prometheusLabel] = filterTagValue
		} else {
			filteredTags[filterTag.name] = filterTagValue
		}
	}

	return
}

func (t *AzureTagFilter) appendPrometheusLabel(labels prometheus.Labels, tags map[string]*string) prometheus.Labels {
	for tagName, tagValue := range t.filterTags(tags, true) {
		labels[tagName] = tagValue
	}
	return labels
}

func (t *AzureTagFilter) azureTagNameToPrometheusTagName(name string) string {
	return azureTagNameToPrometheusNameRegExp.ReplaceAllLiteralString(name, "_")
}

package model

type LabelPair struct {
	Key   string
	Value string
}

func ToLabelPairs(m map[string]string) []LabelPair {
	var pairs []LabelPair
	for k, v := range m {
		pairs = append(pairs, LabelPair{k, v})
	}
	return pairs
}

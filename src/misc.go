package main

func prefixSlice(prefix string, valueMap []string) (ret []string) {
	for _, value := range valueMap {
		ret = append(ret, prefix + value)
	}
	return
}

package rdpserver

func savedPercent(rawBytes, savedBytes int64) float64 {
	if rawBytes <= 0 || savedBytes <= 0 {
		return 0
	}
	return float64(savedBytes) * 100 / float64(rawBytes)
}

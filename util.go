package bearer

import "net/http"

func goHeadersToBearerHeaders(input http.Header) map[string]string {
	if input == nil {
		return nil
	}
	ret := map[string]string{}
	for key, values := range input {
		// bearer headers only support one value per key
		// so we take the first one and ignore the other ones
		ret[key] = values[0]
	}
	return ret
}

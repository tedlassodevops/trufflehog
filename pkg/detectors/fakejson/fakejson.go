package fakejson

import (
	"context"
	"fmt"
	"io/ioutil"

	// "log"
	"regexp"
	"strings"

	"net/http"

	"github.com/trufflesecurity/trufflehog/pkg/common"
	"github.com/trufflesecurity/trufflehog/pkg/detectors"
	"github.com/trufflesecurity/trufflehog/pkg/pb/detectorspb"
)

type Scanner struct{}

// Ensure the Scanner satisfies the interface at compile time
var _ detectors.Detector = (*Scanner)(nil)

var (
	client = common.SaneHttpClient()

	//Make sure that your group is surrounded in boundry characters such as below to reduce false positives
	keyPat = regexp.MustCompile(detectors.PrefixRegex([]string{"fakejson"}) + `\b([a-zA-Z0-9]{22})\b`)
)

// Keywords are used for efficiently pre-filtering chunks.
// Use identifiers in the secret preferably, or the provider name.
func (s Scanner) Keywords() []string {
	return []string{"fakejson"}
}

// FromData will find and optionally verify FakeJSON secrets in a given set of bytes.
func (s Scanner) FromData(ctx context.Context, verify bool, data []byte) (results []detectors.Result, err error) {
	dataStr := string(data)

	matches := keyPat.FindAllStringSubmatch(dataStr, -1)

	for _, match := range matches {
		if len(match) != 2 {
			continue
		}
		resMatch := strings.TrimSpace(match[1])

		s1 := detectors.Result{
			DetectorType: detectorspb.DetectorType_FakeJSON,
			Raw:          []byte(resMatch),
		}

		if verify {
			payload := strings.NewReader(fmt.Sprintf(`{"token":"%s","data":{"user_name":"nameFirst","user_email":"internetEmail","_repeat":3}}`, resMatch))
			req, _ := http.NewRequest("POST", "https://app.fakejson.com/q", payload)
			req.Header.Add("Content-Type", "application/json")
			res, err := client.Do(req)
			if err == nil {
				bodyBytes, err := ioutil.ReadAll(res.Body)
				if err == nil {
					bodyString := string(bodyBytes)
					errCode := strings.Contains(bodyString, `"error":`)
					defer res.Body.Close()
					if res.StatusCode >= 200 && res.StatusCode < 300 {
						if errCode {
							s1.Verified = false
						} else {
							s1.Verified = true
						}
					} else {
						//This function will check false positives for common test words, but also it will make sure the key appears 'random' enough to be a real key
						if detectors.IsKnownFalsePositive(resMatch, detectors.DefaultFalsePositives, true) {
							continue
						}
					}
				}
			}
		}

		results = append(results, s1)
	}

	return detectors.CleanResults(results), nil
}
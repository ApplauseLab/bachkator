package quality

type junitTestCase struct {
	Classname string  `xml:"classname,attr"`
	Name      string  `xml:"name,attr"`
	Time      float64 `xml:"time,attr"`
	Failure   []struct {
		Message string `xml:"message,attr"`
	} `xml:"failure"`
	Error []struct {
		Message string `xml:"message,attr"`
	} `xml:"error"`
}

type junitTestSuite struct {
	Tests    int              `xml:"tests,attr"`
	Failures int              `xml:"failures,attr"`
	Errors   int              `xml:"errors,attr"`
	Skipped  int              `xml:"skipped,attr"`
	Time     float64          `xml:"time,attr"`
	Cases    []junitTestCase  `xml:"testcase"`
	Suites   []junitTestSuite `xml:"testsuite"`
}

type junitTestSuites struct {
	Suites []junitTestSuite `xml:"testsuite"`
}

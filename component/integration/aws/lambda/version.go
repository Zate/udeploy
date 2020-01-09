package lambda

import (
	"errors"
	"regexp"

	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/turnerlabs/udeploy/component/app"
)

func extractVersion(instance app.Instance, config *lambda.FunctionConfiguration) (string, string, error) {
	tag := regexp.MustCompile(instance.Task.ImageTagEx)

	matches := tag.FindStringSubmatch(*config.Description)
	if matches == nil || len(matches) < 2 {
		return "", "", errors.New("failed to extract version")
	}

	version := matches[1]
	build := (*config.RevisionId)[0:8]

	if len(matches) > 2 && len(matches[2]) > 0 {
		version = matches[1]
		build = matches[2]
	}

	return version, build, nil
}

package lambda

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"

	"github.com/turnerlabs/udeploy/component/app"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/lambda"
)

// Populate ...
func Populate(instances map[string]app.Instance) (map[string]app.Instance, error) {

	sess := session.New()

	svc := lambda.New(sess)
	cwsvc := cloudwatch.New(sess)

	for key, instance := range instances {

		i, state, err := populateInst(instance, svc, cwsvc)

		if err != nil {
			state.Error = err
		}

		i.SetState(state)

		instances[key] = i
	}

	return instances, nil
}

func populateInst(i app.Instance, svc *lambda.Lambda, cwsvc *cloudwatch.CloudWatch) (app.Instance, app.State, error) {
	i.Task.Definition = app.Definition{
		ID: fmt.Sprintf("%s-%s", i.FunctionName, i.FunctionAlias),
	}

	state := app.NewState()

	ao, err := svc.GetAlias(&lambda.GetAliasInput{
		FunctionName: aws.String(i.FunctionName),
		Name:         aws.String(i.FunctionAlias),
	})
	if err != nil {
		return i, state, err
	}

	fo, err := svc.GetFunction(&lambda.GetFunctionInput{
		FunctionName: ao.AliasArn,
	})
	if err != nil {
		return i, state, err
	}

	version, build, err := extractVersion(i, fo.Configuration)
	if err != nil {
		return i, state, err
	}

	i.Task.Definition.Version = version
	i.Task.Definition.Build = build
	i.Task.Definition.Description = fmt.Sprintf("%s.%s", version, build)

	env := map[string]string{}
	for k, v := range fo.Configuration.Environment.Variables {
		value := *v
		env[k] = value
	}

	i.Task.Definition.Environment = env
	i.Task.Definition.Secrets = map[string]string{}

	n, err := strconv.ParseInt(*fo.Configuration.Version, 10, 64)
	if err != nil {
		return i, state, err
	}

	i.Task.Definition.Revision = n
	i.Task.DesiredCount = 1

	state.Version = i.FormatVersion()

	region, err := getRegion(*fo.Configuration.FunctionArn)
	if err != nil {
		return i, state, err
	}

	i.Links = append(i.Links, app.Link{
		Generated:   true,
		Description: "CloudWatch logs",
		Name:        "logs",
		URL: fmt.Sprintf("https://console.aws.amazon.com/cloudwatch/home?region=%s#logStream:group=/aws/lambda/%s",
			region, *fo.Configuration.FunctionName),
	})

	alarm, err := cwsvc.DescribeAlarms(&cloudwatch.DescribeAlarmsInput{
		AlarmNames: aws.StringSlice([]string{
			buildAlarmName(i.FunctionName),
		}),
	})
	if err != nil {
		return i, state, err
	}

	if alarm.MetricAlarms == nil || len(alarm.MetricAlarms) == 0 {
		return i, state, errors.New("metric alarm missing")
	}

	if *alarm.MetricAlarms[0].StateValue == "ALARM" {
		state.Error = errors.New(*alarm.MetricAlarms[0].StateReason)
	}

	return i, state, nil
}

func getRegion(arn string) (string, error) {
	tag := regexp.MustCompile("([a-z]{2}-[a-z]*-[0-9]{1})")

	matches := tag.FindStringSubmatch(arn)
	if matches == nil {
		return "", errors.New("failed to get region")
	}

	if len(matches) >= 2 && len(matches[1]) > 0 {
		return matches[1], nil
	}

	return "", errors.New("failed to get region")
}

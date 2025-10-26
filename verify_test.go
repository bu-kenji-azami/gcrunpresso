package ecspresso_test

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/smithy-go"
	"github.com/fatih/color"
	"github.com/kayac/ecspresso/v2"
)

var testRoleArns = []struct {
	arn      string
	roleName string
	isValid  bool
}{
	{
		arn:      "arn:aws:iam::123456789012:role/ecsTaskRole",
		roleName: "ecsTaskRole",
		isValid:  true,
	},
	{
		arn:      "arn:aws:iam::123456789012:role/path/to/ecsTaskRole",
		roleName: "ecsTaskRole",
		isValid:  true,
	},
	{
		arn: "arn:aws:iam::123456789012:foo",
	},
	{
		arn: "arn:aws:iam::123456789012:policy/ecsTaskRole",
	},
	{
		arn: "arn:aws:ec2::123456789012:foo/bar",
	},
	{
		arn: "ecsTaskRole",
	},
}

var testImagesIsECR = []struct {
	image string
	isECR bool
}{
	{
		image: "123456789012.dkr.ecr.ap-northeast-1.amazonaws.com/myimage",
		isECR: true,
	},
	{
		image: "123456789012.dkr.ecr.ap-northeast-1.amazonaws.com/myimage:latest",
		isECR: true,
	},
	{
		image: "ubuntu:latest",
		isECR: false,
	},
	{
		image: "123456789012.dkr.ecr.cn-north-1.amazonaws.com.cn/my-app:latest",
		isECR: true,
	},
}

type goPlatform struct {
	arch string
	os   string
}

var testRuntimePlatforms = []struct {
	platform  types.RuntimePlatform
	isFargate bool
	want      goPlatform
}{
	{
		isFargate: false,
		want: goPlatform{
			arch: "amd64",
			os:   "linux",
		},
	},
	{
		platform: types.RuntimePlatform{
			CpuArchitecture: types.CPUArchitectureArm64,
		},
		isFargate: true,
		want: goPlatform{
			arch: "arm64",
			os:   "linux",
		},
	},
	{
		platform: types.RuntimePlatform{
			OperatingSystemFamily: types.OSFamilyWindowsServer2019Core,
		},
		isFargate: true,
		want: goPlatform{
			arch: "amd64",
			os:   "windows",
		},
	},
	{
		platform: types.RuntimePlatform{
			OperatingSystemFamily: types.OSFamilyWindowsServer2022Full,
		},
		isFargate: true,
		want: goPlatform{
			arch: "amd64",
			os:   "windows",
		},
	},
	{
		platform: types.RuntimePlatform{
			CpuArchitecture: types.CPUArchitectureX8664,
		},
		isFargate: false,
		want: goPlatform{
			arch: "amd64",
			os:   "linux",
		},
	},
	{
		platform: types.RuntimePlatform{
			OperatingSystemFamily: types.OSFamilyWindowsServer2019Core,
		},
		isFargate: false,
		want: goPlatform{
			arch: "amd64",
			os:   "windows",
		},
	},
}

func TestNormalizePlatform(t *testing.T) {
	for _, p := range testRuntimePlatforms {
		arch, os := ecspresso.NormalizePlatform(&p.platform, p.isFargate)
		if arch != p.want.arch || os != p.want.os {
			t.Errorf("want arch/os %s/%s but got %s/%s", p.want.arch, p.want.os, arch, os)
		}
	}
}

func TestParseRoleArn(t *testing.T) {
	for _, s := range testRoleArns {
		name, err := ecspresso.ExtractRoleName(s.arn)
		if s.isValid {
			if name != s.roleName {
				t.Errorf("invalid roleName got:%s expected:%s", name, s.roleName)
			}
			if err != nil {
				t.Error("unexpected error", err)
			}
		} else if err == nil {
			t.Errorf("must be failed valdation for %s", s.arn)
		}
	}
}

func TestIsECRImage(t *testing.T) {
	for _, s := range testImagesIsECR {
		isECR := ecspresso.ECRImageURLRegex.MatchString(s.image)
		if isECR != s.isECR {
			t.Errorf("invalid detect ECR image %s got:%t expected:%t", s.image, isECR, s.isECR)
		}
	}
}

func TestVerifyOKResource(t *testing.T) {
	color.NoColor = true
	for _, cache := range []bool{false, true} {
		vs := ecspresso.NewVerifyState(cache)
		for i := range 3 {
			r, err := vs.VerifyResource(context.TODO(), "ok resource", func(_ context.Context) error {
				return nil
			})
			if err != nil {
				t.Error("unexpected error for ok resource", err)
			}
			if r.Name != "ok resource" {
				t.Error("unexpected output for ok resource", r.Name)
			}
			if r.Result != "OK" {
				t.Error("unexpected output [OK] for ok resource", r.Result)
			}
			if cache && i >= 1 {
				if !r.Cached {
					t.Error("unexpected output (cached) for ok resource", r.Cached)
				}
			}
		}
	}
}

func TestVerifyNGResource(t *testing.T) {
	color.NoColor = true
	for _, cache := range []bool{false, true} {
		vs := ecspresso.NewVerifyState(cache)
		for i := range 3 {
			r, err := vs.VerifyResource(context.TODO(), "ng resource", func(_ context.Context) error {
				return errors.New("XXX")
			})
			if err == nil {
				t.Error("error must be returned for ng resource")
			}
			if r.Name != "ng resource" {
				t.Error("unexpected output for ng resource", r.Name)
			}
			if r.Result != "NG" {
				t.Error("unexpected output [NG] for ng resource", r.Result)
			}
			if r.Error == "" {
				t.Error("error must be returned for ng resource", r.Error)
			}
			if cache && i >= 1 {
				if !r.Cached {
					t.Error("unexpected output (cached) for ng resource", r.Cached)
				}
			}
		}
	}
}

func TestVerifySkipResource(t *testing.T) {
	color.NoColor = true
	for _, cache := range []bool{false, true} {
		vs := ecspresso.NewVerifyState(cache)
		for i := range 3 {
			r, err := vs.VerifyResource(context.TODO(), "skip resource", func(_ context.Context) error {
				return ecspresso.ErrSkipVerify("hello")
			})
			if err != nil {
				t.Error("unexpected error for skip resource", err)
			}
			if r.Result != "SKIP" {
				t.Error("unexpected output [SKIP] for skip resource", r.Result)
			}
			if cache && i >= 1 {
				if !r.Cached {
					t.Error("unexpected output (cached) for skip resource", r.Cached)
				}
			}
		}
	}
}

func TestVerifierIsAssumed(t *testing.T) {
	cfg1 := aws.Config{}
	cfg2 := aws.Config{}
	var testCases = []struct {
		exec      *aws.Config
		app       *aws.Config
		isAssumed bool
	}{
		{&cfg1, &cfg2, true},
		{&cfg1, &cfg1, false},
		{&cfg2, &cfg2, false},
		{&cfg2, &cfg1, true},
	}
	for i, c := range testCases {
		v := ecspresso.NewVerifier(c.exec, c.app, &ecspresso.VerifyOption{})
		if v.IsAssumed() != c.isAssumed {
			t.Errorf("unexpected IsAssumed %d expected:%v got:%v", i, c.isAssumed, v.IsAssumed())
		}
	}
}

type mockAPIError struct {
	code string
}

func (e mockAPIError) Error() string {
	return "mock error: " + e.code
}

func (e mockAPIError) ErrorCode() string {
	return e.code
}

func (e mockAPIError) ErrorMessage() string {
	return "mock error: " + e.code
}

func (e mockAPIError) ErrorFault() smithy.ErrorFault {
	return smithy.FaultClient
}

func TestIsPermissionError(t *testing.T) {
	testCases := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "AccessDeniedException",
			err:      mockAPIError{code: "AccessDeniedException"},
			expected: true,
		},
		{
			name:     "UnauthorizedException",
			err:      mockAPIError{code: "UnauthorizedException"},
			expected: true,
		},
		{
			name:     "Forbidden",
			err:      mockAPIError{code: "Forbidden"},
			expected: true,
		},
		{
			name:     "AccessDenied",
			err:      mockAPIError{code: "AccessDenied"},
			expected: true,
		},
		{
			name:     "InvalidUserID.NotFound",
			err:      mockAPIError{code: "InvalidUserID.NotFound"},
			expected: true,
		},
		{
			name: "AccessDeniedException wrapped in OperationError",
			err: &smithy.OperationError{
				ServiceID:     "IAM",
				OperationName: "GetRole",
				Err:           mockAPIError{code: "AccessDeniedException"},
			},
			expected: true,
		},
		{
			name: "Other error wrapped in OperationError",
			err: &smithy.OperationError{
				ServiceID:     "IAM",
				OperationName: "GetRole",
				Err:           mockAPIError{code: "ValidationException"},
			},
			expected: false,
		},
		{
			name:     "Other error",
			err:      mockAPIError{code: "ValidationException"},
			expected: false,
		},
		{
			name:     "Regular error",
			err:      errors.New("regular error"),
			expected: false,
		},
		{
			name:     "Nil error",
			err:      nil,
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ecspresso.IsPermissionError(tc.err)
			if result != tc.expected {
				t.Errorf("isPermissionError(%v) = %v, expected %v", tc.err, result, tc.expected)
			}
		})
	}
}

func TestVerifyWarnResource(t *testing.T) {
	color.NoColor = true
	for _, cache := range []bool{false, true} {
		vs := ecspresso.NewVerifyState(cache)
		for i := range 3 {
			r, err := vs.VerifyResource(context.TODO(), "warn resource", func(_ context.Context) error {
				// Simulate AWS permission error
				mockErr := mockAPIError{code: "AccessDeniedException"}
				return ecspresso.WrapPermissionError(mockErr)
			})
			if err != nil {
				t.Error("unexpected error for warn resource", err)
			}
			if r.Name != "warn resource" {
				t.Error("unexpected output for warn resource", r.Name)
			}
			if r.Result != "WARN" {
				t.Error("unexpected output [WARN] for warn resource", r.Result)
			}
			if r.Error == "" {
				t.Error("error message must be set for warn resource", r.Error)
			}
			if cache && i >= 1 {
				if !r.Cached {
					t.Error("unexpected output (cached) for warn resource", r.Cached)
				}
			}
		}
	}
}

func TestWrapPermissionError(t *testing.T) {
	testCases := []struct {
		name     string
		err      error
		wantType string
	}{
		{
			name:     "Permission error gets wrapped",
			err:      mockAPIError{code: "AccessDeniedException"},
			wantType: "ErrPermissionDenied",
		},
		{
			name:     "Non-permission error stays unchanged",
			err:      mockAPIError{code: "ValidationException"},
			wantType: "mockAPIError",
		},
		{
			name:     "Nil error stays nil",
			err:      nil,
			wantType: "nil",
		},
		{
			name:     "Regular error stays unchanged",
			err:      errors.New("regular error"),
			wantType: "*errors.errorString",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ecspresso.WrapPermissionError(tc.err)

			switch tc.wantType {
			case "nil":
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
			case "ErrPermissionDenied":
				var permErr ecspresso.ErrPermissionDenied
				if !errors.As(result, &permErr) {
					t.Errorf("expected ErrPermissionDenied, got %T", result)
				}
			case "mockAPIError":
				var mockErr mockAPIError
				if !errors.As(result, &mockErr) {
					t.Errorf("expected mockAPIError, got %T", result)
				}
			case "*errors.errorString":
				if result.Error() != tc.err.Error() {
					t.Errorf("expected same error, got %v", result)
				}
			}
		})
	}
}

package k8s

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"k8s.io/client-go/rest"
)

const eksTokenPrefix = "k8s-aws-v1."

// BuildEKSConfig creates a *rest.Config for an EKS cluster using IAM auth.
// Works with any AWS credential source: Fargate task role, IRSA, EC2 instance
// profile, or environment variables.
func BuildEKSConfig(clusterName, region string) (*rest.Config, error) {
	ctx := context.Background()

	opts := []func(*awsconfig.LoadOptions) error{}
	if region != "" {
		opts = append(opts, awsconfig.WithRegion(region))
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}

	// Describe the EKS cluster to get endpoint and CA
	eksClient := eks.NewFromConfig(cfg)
	out, err := eksClient.DescribeCluster(ctx, &eks.DescribeClusterInput{
		Name: &clusterName,
	})
	if err != nil {
		return nil, fmt.Errorf("describing EKS cluster %q: %w", clusterName, err)
	}

	cluster := out.Cluster
	if cluster.Endpoint == nil || cluster.CertificateAuthority == nil {
		return nil, fmt.Errorf("EKS cluster %q missing endpoint or CA", clusterName)
	}

	caData, err := base64.StdEncoding.DecodeString(*cluster.CertificateAuthority.Data)
	if err != nil {
		return nil, fmt.Errorf("decoding CA data: %w", err)
	}

	stsClient := sts.NewFromConfig(cfg)

	restConfig := &rest.Config{
		Host: *cluster.Endpoint,
		TLSClientConfig: rest.TLSClientConfig{
			CAData: caData,
		},
		WrapTransport: func(rt http.RoundTripper) http.RoundTripper {
			return &eksTokenTransport{
				base:        rt,
				stsClient:   stsClient,
				clusterName: clusterName,
			}
		},
	}

	return restConfig, nil
}

// eksTokenTransport injects a fresh EKS bearer token into each K8s API request.
type eksTokenTransport struct {
	base        http.RoundTripper
	stsClient   *sts.Client
	clusterName string
	cachedToken string
	tokenExpiry time.Time
}

func (t *eksTokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	token, err := t.getToken(req.Context())
	if err != nil {
		return nil, fmt.Errorf("getting EKS token: %w", err)
	}
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "Bearer "+token)
	return t.base.RoundTrip(req)
}

func (t *eksTokenTransport) getToken(ctx context.Context) (string, error) {
	if t.cachedToken != "" && time.Now().Add(60*time.Second).Before(t.tokenExpiry) {
		return t.cachedToken, nil
	}

	presignClient := sts.NewPresignClient(t.stsClient)
	presigned, err := presignClient.PresignGetCallerIdentity(ctx, &sts.GetCallerIdentityInput{},
		func(o *sts.PresignOptions) {
			o.ClientOptions = append(o.ClientOptions, func(so *sts.Options) {
				// Add x-k8s-aws-id header before signing so it becomes part of the signed URL
				so.APIOptions = append(so.APIOptions, smithyhttp.AddHeaderValue("x-k8s-aws-id", t.clusterName))
			})
		},
	)
	if err != nil {
		return "", fmt.Errorf("presigning GetCallerIdentity: %w", err)
	}

	token := eksTokenPrefix + base64.RawURLEncoding.EncodeToString([]byte(presigned.URL))
	t.cachedToken = token
	t.tokenExpiry = time.Now().Add(14 * time.Minute)

	return token, nil
}

// IsEKSContext returns true if the kubeconfig context looks like an EKS cluster.
func IsEKSContext(context string) bool {
	lower := strings.ToLower(context)
	return strings.Contains(lower, "eks") || strings.Contains(lower, "aws") || strings.Contains(lower, "arn:")
}

/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package annotations

import (
	"testing"

	apiv1 "k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"k8s.io/ingress-nginx/internal/ingress/annotations/parser"
	"k8s.io/ingress-nginx/internal/ingress/defaults"
	"k8s.io/ingress-nginx/internal/ingress/resolver"
)

var (
	annotationSecureUpstream       = parser.GetAnnotationWithPrefix("secure-backends")
	annotationSecureVerifyCACert   = parser.GetAnnotationWithPrefix("secure-verify-ca-secret")
	annotationUpsMaxFails          = parser.GetAnnotationWithPrefix("upstream-max-fails")
	annotationUpsFailTimeout       = parser.GetAnnotationWithPrefix("upstream-fail-timeout")
	annotationPassthrough          = parser.GetAnnotationWithPrefix("ssl-passthrough")
	annotationAffinityType         = parser.GetAnnotationWithPrefix("affinity")
	annotationCorsEnabled          = parser.GetAnnotationWithPrefix("enable-cors")
	annotationCorsAllowOrigin      = parser.GetAnnotationWithPrefix("cors-allow-origin")
	annotationCorsAllowMethods     = parser.GetAnnotationWithPrefix("cors-allow-methods")
	annotationCorsAllowHeaders     = parser.GetAnnotationWithPrefix("cors-allow-headers")
	annotationCorsAllowCredentials = parser.GetAnnotationWithPrefix("cors-allow-credentials")
	defaultCorsMethods             = "GET, PUT, POST, DELETE, PATCH, OPTIONS"
	defaultCorsHeaders             = "DNT,X-CustomHeader,Keep-Alive,User-Agent,X-Requested-With,If-Modified-Since,Cache-Control,Content-Type,Authorization"
	annotationAffinityCookieName   = parser.GetAnnotationWithPrefix("session-cookie-name")
	annotationAffinityCookieHash   = parser.GetAnnotationWithPrefix("session-cookie-hash")
	annotationUpstreamHashBy       = parser.GetAnnotationWithPrefix("upstream-hash-by")
)

type mockCfg struct {
	resolver.Mock
	MockSecrets  map[string]*apiv1.Secret
	MockServices map[string]*apiv1.Service
}

func (m mockCfg) GetDefaultBackend() defaults.Backend {
	return defaults.Backend{}
}

func (m mockCfg) GetSecret(name string) (*apiv1.Secret, error) {
	return m.MockSecrets[name], nil
}

func (m mockCfg) GetService(name string) (*apiv1.Service, error) {
	return m.MockServices[name], nil
}

func (m mockCfg) GetAuthCertificate(name string) (*resolver.AuthSSLCert, error) {
	if secret, _ := m.GetSecret(name); secret != nil {
		return &resolver.AuthSSLCert{
			Secret:     name,
			CAFileName: "/opt/ca.pem",
			PemSHA:     "123",
		}, nil
	}
	return nil, nil
}

func buildIngress() *extensions.Ingress {
	defaultBackend := extensions.IngressBackend{
		ServiceName: "default-backend",
		ServicePort: intstr.FromInt(80),
	}

	return &extensions.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: apiv1.NamespaceDefault,
		},
		Spec: extensions.IngressSpec{
			Backend: &extensions.IngressBackend{
				ServiceName: "default-backend",
				ServicePort: intstr.FromInt(80),
			},
			Rules: []extensions.IngressRule{
				{
					Host: "foo.bar.com",
					IngressRuleValue: extensions.IngressRuleValue{
						HTTP: &extensions.HTTPIngressRuleValue{
							Paths: []extensions.HTTPIngressPath{
								{
									Path:    "/foo",
									Backend: defaultBackend,
								},
							},
						},
					},
				},
			},
		},
	}
}

func TestSecureUpstream(t *testing.T) {
	ec := NewAnnotationExtractor(mockCfg{})
	ing := buildIngress()

	fooAnns := []struct {
		annotations map[string]string
		er          bool
	}{
		{map[string]string{annotationSecureUpstream: "true"}, true},
		{map[string]string{annotationSecureUpstream: "false"}, false},
		{map[string]string{annotationSecureUpstream + "_no": "true"}, false},
		{map[string]string{}, false},
		{nil, false},
	}

	for _, foo := range fooAnns {
		ing.SetAnnotations(foo.annotations)
		r := ec.Extract(ing).SecureUpstream
		if r.Secure != foo.er {
			t.Errorf("Returned %v but expected %v", r, foo.er)
		}
	}
}

func TestSecureVerifyCACert(t *testing.T) {
	ec := NewAnnotationExtractor(mockCfg{
		MockSecrets: map[string]*apiv1.Secret{
			"default/secure-verify-ca": {
				ObjectMeta: metav1.ObjectMeta{
					Name: "secure-verify-ca",
				},
			},
		},
	})

	anns := []struct {
		it          int
		annotations map[string]string
		exists      bool
	}{
		{1, map[string]string{annotationSecureUpstream: "true", annotationSecureVerifyCACert: "not"}, false},
		{2, map[string]string{annotationSecureUpstream: "false", annotationSecureVerifyCACert: "secure-verify-ca"}, false},
		{3, map[string]string{annotationSecureUpstream: "true", annotationSecureVerifyCACert: "secure-verify-ca"}, true},
		{4, map[string]string{annotationSecureUpstream: "true", annotationSecureVerifyCACert + "_not": "secure-verify-ca"}, false},
		{5, map[string]string{annotationSecureUpstream: "true"}, false},
		{6, map[string]string{}, false},
		{7, nil, false},
	}

	for _, ann := range anns {
		ing := buildIngress()
		ing.SetAnnotations(ann.annotations)
		su := ec.Extract(ing).SecureUpstream
		if (su.CACert.CAFileName != "") != ann.exists {
			t.Errorf("Expected exists was %v on iteration %v", ann.exists, ann.it)
		}
	}
}

func TestHealthCheck(t *testing.T) {
	ec := NewAnnotationExtractor(mockCfg{})
	ing := buildIngress()

	fooAnns := []struct {
		annotations map[string]string
		eumf        int
		euft        int
	}{
		{map[string]string{annotationUpsMaxFails: "3", annotationUpsFailTimeout: "10"}, 3, 10},
		{map[string]string{annotationUpsMaxFails: "3"}, 3, 0},
		{map[string]string{annotationUpsFailTimeout: "10"}, 0, 10},
		{map[string]string{}, 0, 0},
		{nil, 0, 0},
	}

	for _, foo := range fooAnns {
		ing.SetAnnotations(foo.annotations)
		r := ec.Extract(ing).HealthCheck

		if r.FailTimeout != foo.euft {
			t.Errorf("Returned %d but expected %d for FailTimeout", r.FailTimeout, foo.euft)
		}

		if r.MaxFails != foo.eumf {
			t.Errorf("Returned %d but expected %d for MaxFails", r.MaxFails, foo.eumf)
		}
	}
}

func TestSSLPassthrough(t *testing.T) {
	ec := NewAnnotationExtractor(mockCfg{})
	ing := buildIngress()

	fooAnns := []struct {
		annotations map[string]string
		er          bool
	}{
		{map[string]string{annotationPassthrough: "true"}, true},
		{map[string]string{annotationPassthrough: "false"}, false},
		{map[string]string{annotationPassthrough + "_no": "true"}, false},
		{map[string]string{}, false},
		{nil, false},
	}

	for _, foo := range fooAnns {
		ing.SetAnnotations(foo.annotations)
		r := ec.Extract(ing).SSLPassthrough
		if r != foo.er {
			t.Errorf("Returned %v but expected %v", r, foo.er)
		}
	}
}

func TestUpstreamHashBy(t *testing.T) {
	ec := NewAnnotationExtractor(mockCfg{})
	ing := buildIngress()

	fooAnns := []struct {
		annotations map[string]string
		er          string
	}{
		{map[string]string{annotationUpstreamHashBy: "$request_uri"}, "$request_uri"},
		{map[string]string{annotationUpstreamHashBy: "false"}, "false"},
		{map[string]string{annotationUpstreamHashBy + "_no": "true"}, ""},
		{map[string]string{}, ""},
		{nil, ""},
	}

	for _, foo := range fooAnns {
		ing.SetAnnotations(foo.annotations)
		r := ec.Extract(ing).UpstreamHashBy
		if r != foo.er {
			t.Errorf("Returned %v but expected %v", r, foo.er)
		}
	}
}

func TestAffinitySession(t *testing.T) {
	ec := NewAnnotationExtractor(mockCfg{})
	ing := buildIngress()

	fooAnns := []struct {
		annotations  map[string]string
		affinitytype string
		hash         string
		name         string
	}{
		{map[string]string{annotationAffinityType: "cookie", annotationAffinityCookieHash: "md5", annotationAffinityCookieName: "route"}, "cookie", "md5", "route"},
		{map[string]string{annotationAffinityType: "cookie", annotationAffinityCookieHash: "xpto", annotationAffinityCookieName: "route1"}, "cookie", "md5", "route1"},
		{map[string]string{annotationAffinityType: "cookie", annotationAffinityCookieHash: "", annotationAffinityCookieName: ""}, "cookie", "md5", "INGRESSCOOKIE"},
		{map[string]string{}, "", "", ""},
		{nil, "", "", ""},
	}

	for _, foo := range fooAnns {
		ing.SetAnnotations(foo.annotations)
		r := ec.Extract(ing).SessionAffinity
		t.Logf("Testing pass %v %v %v", foo.affinitytype, foo.hash, foo.name)

		if r.Cookie.Hash != foo.hash {
			t.Errorf("Returned %v but expected %v for Hash", r.Cookie.Hash, foo.hash)
		}

		if r.Cookie.Name != foo.name {
			t.Errorf("Returned %v but expected %v for Name", r.Cookie.Name, foo.name)
		}
	}
}

func TestCors(t *testing.T) {
	ec := NewAnnotationExtractor(mockCfg{})
	ing := buildIngress()

	fooAnns := []struct {
		annotations map[string]string
		corsenabled bool
		methods     string
		headers     string
		origin      string
		credentials bool
	}{
		{map[string]string{annotationCorsEnabled: "true"}, true, defaultCorsMethods, defaultCorsHeaders, "*", true},
		{map[string]string{annotationCorsEnabled: "true", annotationCorsAllowMethods: "POST, GET, OPTIONS", annotationCorsAllowHeaders: "$nginx_version", annotationCorsAllowCredentials: "false"}, true, "POST, GET, OPTIONS", defaultCorsHeaders, "*", false},
		{map[string]string{annotationCorsEnabled: "true", annotationCorsAllowCredentials: "false"}, true, defaultCorsMethods, defaultCorsHeaders, "*", false},
		{map[string]string{}, false, defaultCorsMethods, defaultCorsHeaders, "*", true},
		{nil, false, defaultCorsMethods, defaultCorsHeaders, "*", true},
	}

	for _, foo := range fooAnns {
		ing.SetAnnotations(foo.annotations)
		r := ec.Extract(ing).CorsConfig
		t.Logf("Testing pass %v %v %v %v %v", foo.corsenabled, foo.methods, foo.headers, foo.origin, foo.credentials)

		if r.CorsEnabled != foo.corsenabled {
			t.Errorf("Returned %v but expected %v for Cors Enabled", r.CorsEnabled, foo.corsenabled)
		}

		if r.CorsAllowHeaders != foo.headers {
			t.Errorf("Returned %v but expected %v for Cors Headers", r.CorsAllowHeaders, foo.headers)
		}

		if r.CorsAllowMethods != foo.methods {
			t.Errorf("Returned %v but expected %v for Cors Methods", r.CorsAllowMethods, foo.methods)
		}

		if r.CorsAllowOrigin != foo.origin {
			t.Errorf("Returned %v but expected %v for Cors Methods", r.CorsAllowOrigin, foo.origin)
		}

		if r.CorsAllowCredentials != foo.credentials {
			t.Errorf("Returned %v but expected %v for Cors Methods", r.CorsAllowCredentials, foo.credentials)
		}

	}
}

/*
func TestMergeLocationAnnotations(t *testing.T) {
	// initial parameters
	keys := []string{"BasicDigestAuth", "CorsConfig", "ExternalAuth", "RateLimit", "Redirect", "Rewrite", "Whitelist", "Proxy", "UsePortInRedirects"}

	loc := ingress.Location{}
	annotations := &Ingress{
		BasicDigestAuth:    &auth.Config{},
		CorsConfig:         &cors.Config{},
		ExternalAuth:       &authreq.Config{},
		RateLimit:          &ratelimit.Config{},
		Redirect:           &redirect.Config{},
		Rewrite:            &rewrite.Config{},
		Whitelist:          &ipwhitelist.SourceRange{},
		Proxy:              &proxy.Config{},
		UsePortInRedirects: true,
	}

	// create test table
	type fooMergeLocationAnnotationsStruct struct {
		fName string
		er    interface{}
	}
	fooTests := []fooMergeLocationAnnotationsStruct{}
	for name, value := range keys {
		fva := fooMergeLocationAnnotationsStruct{name, value}
		fooTests = append(fooTests, fva)
	}

	// execute test
	MergeWithLocation(&loc, annotations)

	// check result
	for _, foo := range fooTests {
		fv := reflect.ValueOf(loc).FieldByName(foo.fName).Interface()
		if !reflect.DeepEqual(fv, foo.er) {
			t.Errorf("Returned %v but expected %v for the field %s", fv, foo.er, foo.fName)
		}
	}
	if _, ok := annotations[DeniedKeyName]; ok {
		t.Errorf("%s should be removed after mergeLocationAnnotations", DeniedKeyName)
	}
}
*/

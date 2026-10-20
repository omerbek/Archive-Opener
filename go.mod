module arsivacici

go 1.24.7

replace (
	golang.org/x/crypto => github.com/golang/crypto v0.33.0
	golang.org/x/exp => github.com/golang/exp v0.0.0-20250210185358-939b2ce775ac
	golang.org/x/mod => github.com/golang/mod v0.23.0
	golang.org/x/net => github.com/golang/net v0.35.0
	golang.org/x/sync => github.com/golang/sync v0.11.0
	golang.org/x/sys => github.com/golang/sys v0.30.0
	golang.org/x/text => github.com/golang/text v0.22.0
	golang.org/x/tools => github.com/golang/tools v0.30.0
)

replace gopkg.in/yaml.v3 => ./stubs/gopkg_in_yaml_v3

replace gopkg.in/check.v1 => ./stubs/gopkg_in_check_v1

replace gopkg.in/yaml.v2 => ./stubs/gopkg_in_yaml_v2

replace gopkg.in/Knetic/govaluate.v3 => github.com/Knetic/govaluate v3.0.0+incompatible

replace go4.org => github.com/go4org/go4 v0.0.0-20200411211856-f5505b9728dd

replace cloud.google.com/go => ./stubs/cloud_google_com_go

replace cloud.google.com/go/storage => ./stubs/cloud_google_com_go_storage

replace golang.org/x/oauth2 => ./stubs/golang_org_x_oauth2

replace google.golang.org/api => ./stubs/google_golang_org_api

replace golang.org/x/term => ./stubs/golang_org_x_term

replace golang.org/x/telemetry => ./stubs/golang_org_x_telemetry

require (
	github.com/bodgit/sevenzip v1.6.0
	github.com/klauspost/compress v1.17.9
	github.com/lxn/walk v0.0.0-20210112085537-c389da54e794
	github.com/lxn/win v0.0.0-20210218163916-a377121e959e
	github.com/nwaples/rardecode/v2 v2.4.5
	github.com/ulikunitz/xz v0.5.17
	golang.org/x/crypto v0.33.0
	golang.org/x/sys v0.30.0
	golang.org/x/text v0.22.0
)

require (
	github.com/andybalholm/brotli v1.1.1 // indirect
	github.com/bodgit/plumbing v1.3.0 // indirect
	github.com/bodgit/windows v1.0.1 // indirect
	github.com/hashicorp/errwrap v1.0.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/pierrec/lz4/v4 v4.1.21 // indirect
	go4.org v0.0.0-20200411211856-f5505b9728dd // indirect
	gopkg.in/Knetic/govaluate.v3 v3.0.0-00010101000000-000000000000 // indirect
)

// Package cfn embeds the CloudFormation template used to provision the
// Terraform remote-state backend.
package cfn

import _ "embed"

// Template is the raw CloudFormation template body. This is the fixed
// contract between internal/cfn and internal/backend (see S1-foundation):
// internal/backend.Create must pass this string as TemplateBody. Do not
// change this symbol's name or type.
//
//go:embed cloudformation.yaml
var Template string

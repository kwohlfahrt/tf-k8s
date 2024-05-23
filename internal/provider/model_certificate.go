package provider

type certificateDataSourceModel struct {
	Metadata *certificateMetadata `tfsdk:"metadata"`
	Spec     *certificateSpec     `tfsdk:"spec"`
}

type certificateMetadata struct {
	Name      string `tfsdk:"name"`
	Namespace string `tfsdk:"namespace"`
}

type certificateSpec struct {
	DnsNames   []string  `tfsdk:"dns_names"`
	IssuerRef  issuerRef `tfsdk:"issuer_ref"`
	SecretName string    `tfsdk:"secret_name"`
}

type issuerRef struct {
	Group string `tfsdk:"group"`
	Kind  string `tfsdk:"kind"`
	Name  string `tfsdk:"name"`
}

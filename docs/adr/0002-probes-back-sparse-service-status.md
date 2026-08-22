# Probes back sparse service status without inflating customer availability

When a Service Component has insufficient Customer Requests, fresh Active Probes may support a normal Service Status, while Customer Availability and Probe Availability remain separate measures. This keeps low-traffic services observable without presenting synthetic traffic as real user reliability or making absence of traffic look healthy by default.

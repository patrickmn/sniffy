#!/bin/sh
go test --timeout=5s \
$@ \
github.com/pmylund/sniffy/acl \
github.com/pmylund/sniffy/cert \
github.com/pmylund/sniffy/common \
github.com/pmylund/sniffy/common/queue \
github.com/pmylund/sniffy/dummy \
github.com/pmylund/sniffy/gateway \
github.com/pmylund/sniffy/proxy \
github.com/pmylund/sniffy/sniff \
github.com/pmylund/sniffy/suite \

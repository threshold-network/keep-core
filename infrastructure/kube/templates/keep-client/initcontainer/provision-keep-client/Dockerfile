FROM node:20-slim AS runtime

WORKDIR /tmp

COPY ./package.json /tmp/package.json
COPY ./package-lock.json /tmp/package-lock.json

RUN npm ci --omit=dev --ignore-scripts

COPY ./TokenStaking.json /tmp/TokenStaking.json
COPY ./KeepToken.json /tmp/KeepToken.json
COPY ./KeepRandomBeaconService.json /tmp/KeepRandomBeaconService.json
COPY ./KeepRandomBeaconOperator.json /tmp/KeepRandomBeaconOperator.json

COPY ./keep-client-config-template.toml /tmp/keep-client-config-template.toml

# Regression detector for high-severity dep vulns. ADVISORY ONLY: `|| true`
# is intentional pending a documented allowlist (14 high / 3 critical
# web3-transitive advisories today); infrastructure/** is paths-ignored in
# CI (.github/workflows/client.yml), so this gate cannot regress a build.
RUN npm audit --omit=dev --audit-level=high || true
COPY ./provision-keep-client.js /tmp/provision-keep-client.js

USER node
ENTRYPOINT ["node", "./provision-keep-client.js"]

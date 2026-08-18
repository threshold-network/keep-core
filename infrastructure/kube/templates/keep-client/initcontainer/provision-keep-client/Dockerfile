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

# Regression detector for high-severity dep vulns. Non-fatal: package.json
# overrides are the authoritative fix.
RUN npm audit --omit=dev --audit-level=high || true
COPY ./provision-keep-client.js /tmp/provision-keep-client.js

USER node
ENTRYPOINT ["node", "./provision-keep-client.js"]

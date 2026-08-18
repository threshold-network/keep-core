FROM node:20-slim AS runtime

WORKDIR /tmp

COPY ./package.json /tmp/package.json
COPY ./package-lock.json /tmp/package-lock.json

RUN npm ci --omit=dev

COPY ./TokenStaking.json /tmp/TokenStaking.json
COPY ./KeepToken.json /tmp/KeepToken.json
COPY ./KeepRandomBeaconService.json /tmp/KeepRandomBeaconService.json
COPY ./KeepRandomBeaconOperator.json /tmp/KeepRandomBeaconOperator.json

COPY ./keep-client-config-template.toml /tmp/keep-client-config-template.toml

COPY ./provision-keep-client.js /tmp/provision-keep-client.js

ENTRYPOINT ["node", "./provision-keep-client.js"]

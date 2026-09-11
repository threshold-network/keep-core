FROM node:24.11.1-alpine

RUN apk add --update --no-cache \
    git \
    bash \
    && \
    rm -rf /var/cache/apk/ && mkdir /var/cache/apk/ && \
    rm -rf /usr/share/man

ENV WORK_DIR=/random-beacon
ENV HOME=/home/node
ENV COREPACK_DEFAULT_TO_LATEST=0

# `corepack enable` must run as root: it writes shims into root-owned
# /usr/local/bin. `corepack prepare` (below, after `USER node`) must run as
# node: it writes Corepack's per-user cache under $HOME. `chown` targets only
# $WORK_DIR here because `yarn install` creates node_modules inside it;
# dependency/source ownership is set later via `COPY --chown`.
RUN corepack enable && mkdir -p $WORK_DIR && chown node:node $WORK_DIR
WORKDIR $WORK_DIR
USER node

# Use `https://` instead of unauthenticated `git://` protocol.
RUN git config --global url."https://".insteadOf git://

COPY --chown=node:node package*.json yarn.lock .yarnrc.yml ./
RUN corepack prepare yarn@4.12.0 --activate && yarn install --immutable

COPY --chown=node:node . ./

ENTRYPOINT ["npx", "hardhat"]

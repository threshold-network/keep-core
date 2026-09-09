FROM node:24.11.1-alpine

RUN apk add --update --no-cache \
    git \
    bash \
    && \
    rm -rf /var/cache/apk/ && mkdir /var/cache/apk/ && \
    rm -rf /usr/share/man

ENV WORK_DIR=/random-beacon

RUN mkdir -p $WORK_DIR
WORKDIR $WORK_DIR

# Use `https://` instead of unauthenticated `git://` protocol.
RUN git config --global url."https://".insteadOf git://

COPY package*.json yarn.lock .yarnrc.yml ./
ENV COREPACK_DEFAULT_TO_LATEST=0
RUN corepack enable && corepack prepare yarn@4.12.0 --activate && yarn install --immutable

COPY . ./

ENTRYPOINT ["npx", "hardhat"]

#!/bin/bash
set -e

BUCKET="s3://agentmon-releases/"

if [ -z "$ACCESS_KEY" -o -z "$SECRET_KEY" ]; then
  echo "ACCESS_KEY and SECRET_KEY must be set"
  exit 1
fi

S3CMD="s3cmd --access_key=${ACCESS_KEY} --secret_key=${SECRET_KEY}"

tools=(s3cmd)
for tool in ${tools[@]}; do
  if ! which -s ${tool}; then
    continue
  fi
  tools=( "${tools[@]/${tool}}" )
done

if [ ${#tools} -ne 0 ]; then
  echo "The following tools are missing from \$PATH and are required to run this script"
  echo -e "\t${tools[@]}"
  echo
  exit 1
fi

${S3CMD} put -P $1 ${BUCKET}
echo "https://agentmon-releases.s3.amazonaws.com/$1" >> latest
${S3CMD} put -P latest ${BUCKET}
rm latest

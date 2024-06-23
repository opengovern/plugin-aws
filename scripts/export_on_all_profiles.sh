#!/bin/bash
set -e
set -x

# Check if the first argument is --continue
if [ "$1" == "--continue" ]; then
  CONTINUE="true"
  shift
else
  CONTINUE="false"
fi

PROFILES=$(aws configure list-profiles)

# Iterate over each profile
for PROFILE in $PROFILES
do
  if [ "$CONTINUE" == "true" ] && [ -s "$PROFILE.csv" ]; then
    echo "Skipping $PROFILE because $PROFILE.csv exists"
    continue
  fi

  echo "Running Kaytu for profile: $PROFILE"
  kaytu $@ --profile $PROFILE --output csv > .temp-kaytu.csv
  mv .temp-kaytu.csv $PROFILE.csv
  echo "Kaytu finished optimizations for profile: $PROFILE"
done

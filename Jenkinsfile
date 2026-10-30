#!/usr/bin/env groovy

properties([
  disableConcurrentBuilds(),
])

node() {
  deleteDir()

  vault = new be.kuleuven.icts.Vault()

  vault.withVaultSecrets([
    path: "secret/projects/icts-p-lnx/jenkins/team-ceif-onderzoek/irods/iron_integration_tests_pipeline",
    secretValues: [
      [$class: 'VaultSecretValue', envVar: 'IRON_INTEGRATION_TEST_CONFIG', vaultKey: 'value'],
    ]]) {

    sh '''
      set +x
      printf '%s' "$IRON_INTEGRATION_TEST_CONFIG" > irods_environment.json
    '''

    buildGo {
      noNode = true
      name = "iron"

      // Disable updates
      ldflags = "-X main.updateSlug="

      // Pass IRON_INTEGRATION_TEST_CONFIG to the build
      testflags = "' -e 'IRON_INTEGRATION_TEST_CONFIG=irods_environment.json"

      rpm=true
      rpm_params=[
        repository: "icts-p-coz-rpm-local/common",
      ]
    }
  }
}

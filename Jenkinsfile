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

    buildGo {
      noNode = true
      name = "iron"

      // Disable updates
      ldflags = "-X main.updateSlug="

      // Write out irods test environment file
      prepare = {
	 sh '''
           set -x
           echo $IRON_INTEGRATION_TEST_CONFIG > irods_environment.json
         '''
      }

      // Pass IRON_INTEGRATION_TEST_CONFIG to the build
      testflags = "' -e 'IRON_INTEGRATION_TEST_CONFIG=/src/irods_environment.json"

      rpm=true
      rpm_params=[
        repository: "icts-p-coz-rpm-local/common",
      ]
    }
  }
}

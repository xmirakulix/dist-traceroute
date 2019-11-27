<template>
  <v-container class="fill-height" fluid>
    <v-row align="center" justify="center">
      <v-col cols="12" sm="8" md="6">
        <v-card class="elevation-12">
          <v-toolbar color="primary" dark flat>
            <v-toolbar-title>disttrace Webinterface Login</v-toolbar-title>
          </v-toolbar>
          <v-form @submit.prevent="login">
            <v-card-text>
              <v-container fluid>
                <v-row no-gutters>
                  <v-col sm="9">
                    <v-text-field
                      label="Login"
                      name="login"
                      prepend-icon="fas fa-user"
                      type="text"
                      v-model="creds.user"
                      :error="failed"
                      :disabled="waiting"
                      autofocus
                  /></v-col>
                </v-row>
                <v-row no-gutters>
                  <v-col sm="9">
                    <v-text-field
                      id="password"
                      label="Password"
                      name="password"
                      prepend-icon="fas fa-lock"
                      type="password"
                      v-model="creds.password"
                      :error="failed"
                      :disabled="waiting"
                    />
                  </v-col>
                </v-row>
              </v-container>
            </v-card-text>
            <v-card-actions>
              <v-spacer />
              <v-btn
                color="secondary"
                @click="login"
                type="submit"
                :disabled="waiting"
              >
                Login
                <v-progress-circular
                  v-if="waiting"
                  indeterminate
                  class="ml-2"
                  size="16"
                  width="2"
                />
              </v-btn>
            </v-card-actions>
          </v-form>
        </v-card>
      </v-col>
    </v-row>
  </v-container>
</template>

<script>
import { mapActions, mapGetters } from "vuex";

export default {
  name: "Login",

  data() {
    return {
      creds: {
        user: "",
        password: ""
      },

      waiting: false,
      failed: false
    };
  },

  props: {
    source: String
  },

  methods: {
    ...mapActions(["fetchAuthToken"]),

    login: function() {
      this.waiting = true;
      this.fetchAuthToken(this.creds)
        .then(() => {})
        .catch(() => {
          this.waiting = false;
          this.failed = true;
        });
    }
  },

  computed: mapGetters(["isAuthorized"])
};
</script>

<style scoped></style>

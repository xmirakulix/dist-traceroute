<template>
  <v-app>
    <v-app-bar app clipped-left color="primary" dark>
      <!-- <v-app-bar-nav-icon @click.stop="drawer = !drawer"></v-app-bar-nav-icon> -->
      <v-toolbar-title>disttrace Webinterface</v-toolbar-title>
      <v-spacer></v-spacer>
      <v-btn v-if="isAuthorized" text @click="logout">
        Sign out
        <v-icon class="ml-2">fas fa-sign-out-alt</v-icon>
      </v-btn>
    </v-app-bar>

    <v-navigation-drawer
      app
      clipped
      permanent
      expand-on-hover
      v-if="isAuthorized"
    >
      <v-list>
        <v-list-item :to="{ name: 'home' }" exact color="primary">
          <v-list-item-action>
            <v-icon>fas fa-home</v-icon>
          </v-list-item-action>
          <v-list-item-title>Home</v-list-item-title>
        </v-list-item>
        <v-list-item :to="{ name: 'history' }" color="primary">
          <v-list-item-action>
            <v-icon>fas fa-history</v-icon>
          </v-list-item-action>
          <v-list-item-title>Trace Results</v-list-item-title>
        </v-list-item>
        <v-list-item
          :to="{ name: 'graph', params: { dest: -1 } }"
          color="primary"
        >
          <v-list-item-action>
            <v-icon>fas fa-project-diagram</v-icon>
          </v-list-item-action>
          <v-list-item-title>Trace Graph</v-list-item-title>
        </v-list-item>
      </v-list>
    </v-navigation-drawer>

    <v-content>
      <v-container v-if="isAuthorized">
        <router-view />
      </v-container>
      <v-container v-else>
        <Login />
      </v-container>
    </v-content>

    <v-footer color="primary" app>
      <span class="white--text">&copy; 2019</span>
    </v-footer>
  </v-app>
</template>

<script>
import { mapGetters, mapActions, mapMutations } from "vuex";
import Login from "@/components/Login";

export default {
  name: "app",
  data: function() {
    return {
      drawer: null
    };
  },

  components: { Login },

  methods: {
    ...mapActions(["fetchAuthToken", "removeAuth"]),
    ...mapMutations(["unsetToken"]),

    login: function() {
      this.fetchAuthToken({ user: "admin", password: "123" });
    },
    logout: function() {
      this.unsetToken();
    }
  },

  computed: {
    ...mapGetters(["getAuthClaims", "isAuthorized"])
  }
};
</script>

<style scoped></style>

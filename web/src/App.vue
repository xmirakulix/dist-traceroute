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
      v-model="drawer"
      app
      clipped
      permanent
      expand-on-hover
      v-if="isAuthorized"
    >
      <v-list>
        <template v-for="(item, i) in items">
          <!-- Menu entries -->
          <v-list-item v-if="item.icon" :to="item.to" :key="i" color="primary">
            <v-list-item-action>
              <v-icon>{{ item.icon }}</v-icon>
            </v-list-item-action>
            <v-list-item-title>{{ item.text }}</v-list-item-title>
          </v-list-item>

          <!-- Headings -->
          <v-list-item v-else-if="item.heading" :key="i" dense class="mt-4">
            <v-list-item-title>
              {{ item.heading }}
            </v-list-item-title>
          </v-list-item>

          <!-- Dividers -->
          <v-divider v-else-if="item.divider" :key="i" dark class="mb-2" />
        </template>
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
      <v-row no-gutters justify="space-between">
        <v-col v-show="isAuthorized" class="white--text">
          Uptime: {{ uptime }}
        </v-col>
        <v-col class="white--text" align="right">
          &copy; 2019
          <a
            class="white--text"
            style="text-decoration: none;"
            href="https://github.com/xmirakulix/dist-traceroute/"
          >
            dist-traceroute
          </a>
        </v-col>
      </v-row>
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
      drawer: null,
      items: [
        { icon: "fas fa-home", text: "Home", to: "/" },
        { icon: "fas fa-history", text: "Trace Results", to: "/history" },
        {
          icon: "fa-project-diagram",
          text: "Trace Graph",
          to: { name: "graph", params: { dest: -1 } }
        },
        { heading: "Configuration" },
        { divider: true },
        { icon: "fas fa-user-cog", text: "Users", to: "/config/users" },
        { icon: "fas fa-server", text: "Slaves", to: "/config/slaves" },
        {
          icon: "fas fa-map-marker-alt",
          text: "Targets",
          to: "/config/targets"
        }
      ]
    };
  },

  components: { Login },

  methods: {
    ...mapActions(["fetchAuthToken", "removeAuth", "fetchStatus"]),
    ...mapMutations(["unsetToken"]),

    login: function() {
      this.fetchAuthToken({ user: "admin", password: "123" });
    },
    logout: function() {
      this.unsetToken();
    }
  },

  computed: {
    ...mapGetters(["getAuthClaims", "isAuthorized", "getStatus"]),

    uptime: function() {
      return this.isAuthorized ? this.getStatus.Uptime : "";
    }
  },

  created: function() {
    // regularly check status -> auto logged out on auth failure
    setInterval(() => {
      this.fetchStatus();
    }, 10 * 1000);
  }
};
</script>

<style scoped></style>

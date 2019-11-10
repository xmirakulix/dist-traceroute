<template>
  <v-app>
    <v-app-bar app clipped-left color="primary" dark>
      <!-- <v-app-bar-nav-icon @click.stop="drawer = !drawer"></v-app-bar-nav-icon> -->
      <v-toolbar-title>disttrace Webinterface</v-toolbar-title>
    </v-app-bar>

    <v-navigation-drawer app clipped permanent expand-on-hover>
      <v-list>
        <v-list-item @click="doAlert('home')">
          <v-list-item-action>
            <v-icon>fas fa-home</v-icon>
          </v-list-item-action>
          <v-list-item-title>Home</v-list-item-title>
        </v-list-item>
        <v-list-item @click="doAlert('contact')">
          <v-list-item-action>
            <v-icon>fas fa-envelope</v-icon>
          </v-list-item-action>
          <v-list-item-title>Contact</v-list-item-title>
        </v-list-item>
      </v-list>
    </v-navigation-drawer>

    <v-content>
      <!-- Header -->
      <v-container>
        <h1>dist-traceroute Master</h1>
        <p>Hi, this is the webservice of the dist-traceroute master service.</p>
        <p>Uptime: {{ getStatus.Uptime }}</p>
      </v-container>

      <!-- List last received traces -->
      <v-container>
        <ListTraces />
      </v-container>
      <!-- current master config -->
      <v-container>
        <h2>Currently loaded master config</h2>
        <code class="d-block">{{ getStatus.CurrentMasterConfig }}</code>
      </v-container>

      <!-- last slave action -->
      <v-container>
        <h2>Last transmitted slave config</h2>
        <code class="d-block">{{ getStatus.LastSlaveConfig }}</code>
        <p>
          {{
            getStatus.LastSlaveConfigTime == "" ||
            getStatus.LastSlaveConfigTime == undefined
              ? ""
              : "(" + getStatus.LastSlaveConfigTime + ")"
          }}
        </p>
      </v-container>
    </v-content>

    <v-footer color="primary" app>
      <span class="white--text">&copy; 2019</span>
    </v-footer>
  </v-app>
</template>

<script>
import { mapGetters, mapActions } from "vuex";
import ListTraces from "./components/ListTraces.vue";

export default {
  name: "app",
  data: function() {
    return {
      drawer: null
    };
  },

  components: {
    ListTraces
  },

  computed: mapGetters(["getStatus"]),

  methods: {
    ...mapActions(["fetchStatus"]),

    doAlert: text => console.log(text)
  },

  created() {
    this.fetchStatus();
  }
};
</script>

<style scope></style>

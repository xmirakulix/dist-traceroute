<template>
  <v-app>
    <v-app-bar app clipped-left color="primary" dark>
      <!-- <v-app-bar-nav-icon @click.stop="drawer = !drawer"></v-app-bar-nav-icon> -->
      <v-toolbar-title>disttrace Webinterface</v-toolbar-title>
    </v-app-bar>

    <v-navigation-drawer app clipped permanent expand-on-hover>
      <v-list>
        <v-list-item @click="goTo('/')">
          <v-list-item-action>
            <v-icon>fas fa-home</v-icon>
          </v-list-item-action>
          <v-list-item-title>Home</v-list-item-title>
        </v-list-item>
        <v-list-item @click="goTo('/history')">
          <v-list-item-action>
            <v-icon>fas fa-history</v-icon>
          </v-list-item-action>
          <v-list-item-title>Trace Results</v-list-item-title>
        </v-list-item>
      </v-list>
    </v-navigation-drawer>

    <v-content>
      <router-view />
    </v-content>

    <v-footer color="primary" app>
      <span class="white--text">&copy; 2019</span>
    </v-footer>
  </v-app>
</template>

<script>
import router from "./router";

export default {
  name: "app",
  data: function() {
    return {
      drawer: null
    };
  },
  methods: {
    goTo: url => router.push(url).catch(() => {})
  }
};
</script>

<style scope></style>

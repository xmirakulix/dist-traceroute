<template>
  <div id="app">
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
        <h1>dist-traceroute Master</h1>
        <p>Hi, this is the webservice of the dist-traceroute master service.</p>
        <p>Uptime: {{ status.Uptime }}</p>

        <h2>Last results received</h2>
        <table>
          <tr>
            <th>nTracerouteId</th>
            <th>strOriginSlave</th>
            <th>strDestination</th>
            <th>dtStart</th>
            <th>nHopCount</th>
          </tr>
          <tr>
            <td>" + fmt.Sprintf("%v", traceID) + "</td>
            <td>" + slaveName + "</td>
            <td>" + destName + "</td>
            <td>" + startTime + "</td>
            <td title='" + detailJSON + "'>
              " + fmt.Sprintf("%v", hopCnt) + "
            </td>
          </tr>
        </table>

        <h2>Currently loaded master config</h2>
        <code class="d-block">{{ status.CurrentMasterConfig }}</code>

        <h2>Last transmitted slave config</h2>

        <code class="d-block">{{ status.LastSlaveConfig }}</code>
        <p>
          {{
            status.LastSlaveConfigTime == "" ||
            status.LastSlaveConfigTime == undefined
              ? ""
              : "(" + status.LastSlaveConfigTime + ")"
          }}
        </p>

        <AddSlave />
      </v-content>

      <v-footer color="primary" app>
        <span class="white--text">&copy; 2019</span>
      </v-footer>
    </v-app>
  </div>
</template>

<script>
import { mapGetters, mapActions } from "vuex";
import AddSlave from "./components/AddSlave.vue";

export default {
  name: "app",
  data: function() {
    return {
      drawer: null
    };
  },

  components: {
    AddSlave
  },

  computed: mapGetters(["status"]),

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

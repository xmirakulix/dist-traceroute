<template>
  <div id="app">
    <title>dist-traceroute Master</title>
    <h1>dist-traceroute Master</h1>
    <p>Hi, this is the webservice of the dist-traceroute master service.</p>
    <p>Uptime: {{ uptime }}</p>

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
        <td title='" + detailJSON + "'>" + fmt.Sprintf("%v", hopCnt) + "</td>
      </tr>
    </table>

    <h2>Currently loaded master config</h2>
    <pre>" + string(masterCfgJSON) + "</pre>

    <h2>Last transmitted slave config</h2>
    <p>Time: " + timeSinceSlaveCfg + "</p>
    <pre>" + lastTransmittedSlaveConfig + "</pre>

    <AddSlave />
  </div>
</template>

<script>
import { mapGetters, mapActions } from "vuex";
import AddSlave from "./components/AddSlave.vue";

export default {
  name: "app",

  components: {
    AddSlave
  },

  computed: mapGetters(["uptime"]),

  methods: {
    ...mapActions(["fetchUptime"])
  },

  created() {
    this.fetchUptime();
  }
};
</script>

<style scope>
h2 {
  margin: 40px 0 0;
}
</style>

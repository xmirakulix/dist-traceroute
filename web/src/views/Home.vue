<template>
  <div class="home">
    <!-- Header -->
    <v-container>
      <h1>
        dist-traceroute Master
        <v-icon style="font-size: 1rem;" @click="fetchStatus()"
          >fas fa-sync</v-icon
        >
      </h1>
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
  </div>
</template>

<script>
import { mapGetters, mapActions } from "vuex";
import ListTraces from "@/components/ListTraces.vue";

export default {
  name: "home",
  components: { ListTraces },
  computed: mapGetters(["getStatus"]),

  methods: mapActions(["fetchStatus"]),

  created() {
    this.fetchStatus();
  }
};
</script>

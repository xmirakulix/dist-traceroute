<template>
  <div class="home">
    <!-- Header -->
    <h1>
      dist-traceroute Master
      <v-icon style="font-size: 1rem;" @click="fetchStatus()"
        >fas fa-sync</v-icon
      >
    </h1>
    <v-container>
      <p>Hi, this is the webservice of the dist-traceroute master service.</p>
      <p class="mb-0">Uptime: {{ getStatus.Uptime }}</p>
    </v-container>

    <!-- List last received traces -->
    <ListTraces />

    <!-- current master config -->
    <h2 class="mt-6">Currently loaded master config</h2>
    <v-container>
      <code class="d-block">{{ getStatus.CurrentMasterConfig }}</code>
    </v-container>

    <!-- last slave action -->
    <h2 class="mt-6">Last transmitted slave config</h2>
    <v-container>
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

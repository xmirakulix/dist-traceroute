<template>
  <div>
    <h2>Last results received</h2>
    <v-data-table
      :headers="headers"
      :items="getTraces"
      :items-per-page="5"
      :hide-default-footer="true"
      :disable-sort="true"
      class="elevation-1"
    ></v-data-table>
  </div>
</template>

<script>
import { mapActions, mapGetters } from "vuex";

export default {
  name: "ListTraces",

  data() {
    return {
      headers: [
        { text: "Time", value: "StartTime" },
        { text: "Slave", value: "SlaveName" },
        { text: "Destination", value: "DestName" },
        { text: "Hops", value: "HopCnt", align: "end" }
      ]
    };
  },

  methods: {
    ...mapActions(["fetchTraces"])
  },
  computed: {
    ...mapGetters(["getTraces"])
  },
  created() {
    this.fetchTraces();
  }
};
</script>

<style scoped></style>

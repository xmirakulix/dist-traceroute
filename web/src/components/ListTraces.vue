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
    >
      <!-- insert graph link -->
      <template v-slot:item.DestName="{ item }">
        <v-tooltip bottom>
          <template v-slot:activator="{ on }">
            <v-icon
              v-on="on"
              small
              class="mr-1"
              color="primary"
              @click="
                $router.push({ name: 'graph', params: { dest: item.DestName } })
              "
            >
              fas fa-project-diagram
            </v-icon>
          </template>
          <span>View graph...</span>
        </v-tooltip>
        <span>{{ item.DestName }}</span>
      </template>

      <!-- insert details tooltip -->
      <template v-slot:item.HopCnt="{ item }">
        <v-tooltip bottom>
          <template v-slot:activator="{ on }">
            <span v-on="on">{{ item.HopCnt }}</span>
          </template>
          <span>{{ item.DetailJSON }}</span>
        </v-tooltip>
      </template>
    </v-data-table>
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
    this.fetchTraces(3);
  }
};
</script>

<style scoped></style>

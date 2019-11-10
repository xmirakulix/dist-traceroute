<template>
  <v-container>
    <h1>Trace Graph for {{ dest }}</h1>
    <v-card>
      <GChart
        v-if="dest != '-1'"
        style="height: 300px;"
        type="Sankey"
        :data="chartData"
        :options="chartOptions"
        :settings="{
          packages: ['sankey']
        }"
      />
    </v-card>
  </v-container>
</template>

<script>
import { GChart } from "vue-google-charts";
import { mapGetters, mapActions } from "vuex";

export default {
  name: "TraceGraph",

  data() {
    return {
      chartOptions: {
        chart: {
          title: "Route to trace destination"
        },
        sankey: {
          iterations: 1000,
          node: { nodePadding: 20 }
        }
      },
      dest: this.$route.params.dest
    };
  },

  watch: {
    $route() {
      this.dest = this.$route.params.dest;
    }
  },

  components: {
    GChart
  },

  methods: mapActions(["fetchGraphData"]),

  computed: {
    ...mapGetters(["getGraphData"]),

    chartData: function() {
      this.fetchGraphData(this.dest);
      var header = [["From", "To", "Weight", "Latency"]];
      return header.concat(this.getGraphData);
    }
  }
};
</script>

<style></style>

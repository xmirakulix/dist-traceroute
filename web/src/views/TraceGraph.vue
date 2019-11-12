<template>
  <div>
    <h1>Trace Graph for {{ dest }}</h1>
    <v-card class="my-6">
      <v-container>
        <v-row justify="end" dense>
          <v-col md="4" sm="6" xs="12">
            <v-slider
              hide-details
              :label="'Hide first ' + skip + ' hops'"
              v-model="skip"
              min="0"
              max="20"
              v-on:end="setSkipVal"
            >
              <template v-slot:prepend>
                <v-icon small class="mt-1">fas fa-eye-slash fa-xs</v-icon>
              </template>
            </v-slider>
          </v-col>
        </v-row>
        <v-row dense>
          <v-col>
            <GChart
              v-if="dest != '-1'"
              type="Sankey"
              :data="chartData"
              :options="chartOptions"
              :settings="{
                packages: ['sankey']
              }"
            />
          </v-col>
        </v-row>
      </v-container>
    </v-card>
  </div>
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
        height: 300,
        sankey: {
          iterations: 1000,
          node: {
            nodePadding: 20,
            interactivity: true
          }
        }
      },
      dest: "",
      skip: 0
    };
  },

  created() {
    this.dest = this.$route.params.dest;
    this.fetchGraphData({ dest: this.dest, skip: this.skip });
  },

  components: {
    GChart
  },

  methods: {
    ...mapActions(["fetchGraphData"]),

    setSkipVal(val) {
      this.fetchGraphData({ dest: this.dest, skip: val });
    }
  },
  computed: {
    ...mapGetters(["getGraphData"]),

    chartData: function() {
      var header = [["From", "To", "Weight", "Latency"]];
      return header.concat(this.getGraphData);
    }
  }
};
</script>

<style></style>

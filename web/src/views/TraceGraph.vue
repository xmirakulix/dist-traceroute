<template>
  <div>
    <h1>Trace Graph</h1>
    <v-container>
      <p>Destination: {{ destID }}</p>
      <p>
        Graph interval:
        {{ dateFormat(new Date(graphStart), "yyyy-mm-dd HH:MM:ss") }}
        to
        {{ dateFormat(new Date(graphEnd), "yyyy-mm-dd HH:MM:ss") }}
      </p>
    </v-container>
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
              color="accent"
              track-color="accent lighten-5"
            >
              <template v-slot:prepend>
                <v-icon color="accent" small class="mt-1"
                  >fas fa-eye-slash fa-xs</v-icon
                >
              </template>
            </v-slider>
          </v-col>
        </v-row>
        <v-row dense>
          <v-col>
            <GChart
              v-if="destID != '-1'"
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
import dateFormat from "dateformat";

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
      destID: "",
      slaveID: "",
      skip: 0,

      interval: [0, 0]
    };
  },

  created() {
    this.destID = this.$route.params.destID;
    this.slaveID = this.$route.params.slaveID;
    this.fetchGraphData({
      destID: this.destID,
      slaveID: this.slaveID,
      skip: 0
    });
  },

  components: {
    GChart
  },

  methods: {
    ...mapActions(["fetchGraphData"]),
    dateFormat,

    setSkipVal(val) {
      this.fetchGraphData({
        destID: this.destID,
        slaveID: this.slaveID,
        skip: val
      });
    }
  },
  computed: {
    ...mapGetters(["getGraphData", "getGraphStart", "getGraphEnd"]),

    chartData: function() {
      var header = [["From", "To", "Weight", "Latency"]];
      return header.concat(this.getGraphData);
    },

    graphStart: function() {
      return new Date(this.getGraphStart).getTime();
    },
    graphEnd: function() {
      return new Date(this.getGraphEnd).getTime();
    }
  }
};
</script>

<style></style>

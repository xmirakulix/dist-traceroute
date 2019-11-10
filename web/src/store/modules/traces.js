import axios from "axios";

const state = {
  traces: [],
  graphData: []
};

const getters = {
  getTraces: state => state.traces,
  getGraphData: state => state.graphData
};

const actions = {
  async fetchTraces({ commit }, limit) {
    try {
      const response = await axios.get(
        `http://localhost:8990/api/traces?_limit=${limit}`
      );
      commit("setTraces", response.data);
    } catch (error) {
      console.log("Error caught: " + error);
    }
  },

  async fetchGraphData({ commit }, destination) {
    try {
      const response = await axios.get(
        `http://localhost:8990/api/graph?_dest=${destination}`
      );
      commit("setGraphData", response.data);
    } catch (error) {
      console.log("Error caught: " + error);
    }
  }
};

const mutations = {
  setTraces: (state, traces) => (state.traces = traces),

  setGraphData: (state, graphData) => (state.graphData = graphData)
};

export default {
  state,
  getters,
  actions,
  mutations
};

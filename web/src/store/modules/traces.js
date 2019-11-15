import axios from "axios";

const state = {
  traces: [],
  graphData: [],
  graphStart: 0,
  graphEnd: 0
};

const getters = {
  getTraces: state => state.traces,
  getGraphData: state => state.graphData,
  getGraphStart: state => state.graphStart,
  getGraphEnd: state => state.graphEnd
};

const actions = {
  async fetchTraces({ commit }, limit) {
    try {
      const response = await axios.get(
        `http://localhost:8990/api/traces?limit=${limit}`
      );
      commit("setTraces", response.data);
    } catch (error) {
      console.log("Error caught: " + error);
    }
  },

  async fetchGraphData({ commit }, payload) {
    try {
      const response = await axios.get(
        `http://localhost:8990/api/graph?dest=${payload.dest}&skip=${payload.skip}`
      );
      commit("setGraphData", response.data.Data);
      commit("setGraphStart", response.data.Start);
      commit("setGraphEnd", response.data.End);
    } catch (error) {
      console.log("Error caught: " + error);
    }
  }
};

const mutations = {
  setTraces: (state, traces) => (state.traces = traces),

  setGraphData: (state, graphData) => (state.graphData = graphData),
  setGraphStart: (state, graphStart) => (state.graphStart = graphStart),
  setGraphEnd: (state, graphEnd) => (state.graphEnd = graphEnd)
};

export default {
  state,
  getters,
  actions,
  mutations
};

import axios from "axios";

const state = () => {
  return {
    traces: [],
    graphData: [],
    graphStart: 0,
    graphEnd: 0
  };
};

const getters = {
  getTraces: state => state.traces,
  getGraphData: state => state.graphData,
  getGraphStart: state => state.graphStart,
  getGraphEnd: state => state.graphEnd
};

const actions = {
  async fetchTraces({ commit, rootGetters }, limit) {
    if (!rootGetters["isAuthorized"]) return;
    try {
      const response = await axios.get(
        `http://localhost:8990/api/traces?limit=${limit}`,
        rootGetters["getAuthHeader"]
      );
      commit("setTraces", response.data);
    } catch (error) {
      console.log("Error caught: " + error);
    }
  },

  async fetchGraphData({ commit, rootGetters }, payload) {
    if (!rootGetters["isAuthorized"]) return;
    try {
      const response = await axios.get(
        `http://localhost:8990/api/graph?dest=${payload.dest}&skip=${payload.skip}`,
        rootGetters["getAuthHeader"]
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

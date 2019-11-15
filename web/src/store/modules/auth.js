import axios from "axios";

const state = () => {
  return {
    token: "123"
  };
};

const getters = {
  getAuthHeader: state => {
    return {
      headers: { Authorization: "Bearer " + state.token }
    };
  }
};

const actions = {
  async fetchAuthToken({ commit, rootGetters }, creds) {
    try {
      const response = await axios.get(
        `http://localhost:8990/api/auth?user=${creds.user}&password=${creds.password}`,
        rootGetters["getAuthHeader"]
      );
      commit("setToken", response.data);
    } catch (error) {
      console.log("Error caught: " + error);
    }
  }
};

const mutations = {
  setToken: (state, token) => (state.token = token)
};

export default {
  state,
  getters,
  actions,
  mutations
};

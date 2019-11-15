import axios from "axios";
import jwtDecode from "jwt-decode";

const state = () => {
  return {
    token: "",
    claims: {}
  };
};

const getters = {
  getAuthHeader: state => {
    return {
      headers: { Authorization: "Bearer " + state.token }
    };
  },

  getAuthClaims: state => state.claims,

  isAuthorized: state => state.token !== ""
};

const actions = {
  fetchAuthToken({ commit, rootGetters }, creds) {
    return new Promise((resolve, reject) => {
      axios
        .get(
          `http://localhost:8990/api/auth?user=${creds.user}&password=${creds.password}`,
          rootGetters["getAuthHeader"]
        )
        .then(res => {
          commit("setToken", res.data);
          resolve(true);
        })
        .catch(err => {
          console.log("fetchAuthToken Error caught: " + err);
          reject(false);
        });
    });
  }
};

const mutations = {
  setToken: (state, token) => {
    state.token = token;
    state.claims = jwtDecode(token);
  },

  unsetToken: state => {
    state.token = "";
    state.claims = {};
  }
};

export default {
  state,
  getters,
  actions,
  mutations
};

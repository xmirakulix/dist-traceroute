import axios from "axios";

const state = () => {
  return {
    users: []
  };
};

const getters = {
  getUsers: state => state.users
};

const actions = {
  async fetchUsers({ commit, rootGetters }, limit) {
    if (!rootGetters["isAuthorized"]) return;
    try {
      const response = await axios.get(
        `http://localhost:8990/api/users?limit=${limit}`,
        rootGetters["getAuthHeader"]
      );
      commit("setUsers", response.data);
    } catch (error) {
      console.log("Error caught: " + error);
    }
  },

  async createUser({ commit, rootGetters }, user) {
    if (!rootGetters["isAuthorized"]) return;
    try {
      const response = await axios.post(
        `http://localhost:8990/api/users?name=${user.Name}&password=${user.Password}&pwNeedsChange=${user.PasswordNeedsChange}`,
        "",
        rootGetters["getAuthHeader"]
      );
      commit("addUser", response.data);
      return response.data;
    } catch (error) {
      console.log("Error caught: " + error);
      return false;
    }
  },

  async updateUser({ commit, dispatch, rootGetters }, user) {
    if (!rootGetters["isAuthorized"]) return;
    try {
      if (user.PasswordChanged) {
        console.log(user.Password);
        user.Password = btoa(user.Password);
        console.log(btoa(user.Password));
        console.log(user.Password);
      }
      const response = await axios.put(
        `http://localhost:8990/api/users`,
        user,
        rootGetters["getAuthHeader"]
      );
      commit("updateUser", response.data);
      dispatch("fetchUsers");
      return response.data;
    } catch (error) {
      console.log("Error caught: " + error);
      return false;
    }
  },

  async deleteUser({ commit, rootGetters }, userID) {
    if (!rootGetters["isAuthorized"]) return;
    try {
      const response = await axios.delete(
        `http://localhost:8990/api/users/${userID}`,
        rootGetters["getAuthHeader"]
      );
      commit("removeUser", response.data.ID);
      return response.data;
    } catch (error) {
      console.log("Error caught: " + error);
      return false;
    }
  }
};

const mutations = {
  setUsers: (state, users) => (state.users = users),
  addUser: (state, user) => state.users.push(user),
  updateUser: (state, user) => {
    state.users = state.users.map(el => (el.ID == user.ID ? user : el));
  },
  removeUser: (state, userID) =>
    (state.users = state.users.filter(el => el.ID != userID))
};

export default {
  state,
  getters,
  actions,
  mutations
};

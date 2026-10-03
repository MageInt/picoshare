"use strict";

export async function settingsPut(settings) {
  return fetch("/api/settings", {
    method: "PUT",
    credentials: "include",
    body: JSON.stringify(settings),
  })
    .then((response) => {
      if (!response.ok) {
        return response.text().then((error) => {
          return Promise.reject(error);
        });
      }
      return Promise.resolve();
    })
    .catch((error) => {
      if (error.message) {
        return Promise.reject(
          "Failed to communicate with server" +
            (error.message ? `: ${error.message}` : "."),
        );
      }
      return Promise.reject(error);
    });
}

export async function apiKeyPost() {
  return fetch("/api/settings/api-key", {
    method: "POST",
    credentials: "include",
  })
    .then((response) => {
      if (!response.ok) {
        return response.text().then((error) => {
          return Promise.reject(error);
        });
      }
      return response.json();
    })
    .catch((error) => {
      if (error.message) {
        return Promise.reject(
          "Failed to communicate with server" +
            (error.message ? `: ${error.message}` : "."),
        );
      }
      return Promise.reject(error);
    });
}

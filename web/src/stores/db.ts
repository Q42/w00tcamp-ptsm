import { initializeApp } from "firebase/app";
import { connectAuthEmulator, getAuth } from "firebase/auth";
import { connectFirestoreEmulator, getFirestore } from "firebase/firestore";
import { connectFunctionsEmulator, getFunctions } from "firebase/functions";
import { getStorage } from "firebase/storage";

import { firebaseConfig } from "./firebase-config";

const app = initializeApp(firebaseConfig);
export const db = getFirestore(app);
export const storage = getStorage(app);
export const auth = getAuth(app);
export const functions = getFunctions(app, "europe-west1");

auth.languageCode = "nl";

// if (envdb === "EMULATOR") {
//   connectFirestoreEmulator(db, "localhost", 8081);
//   connectFunctionsEmulator(functions, "localhost", 5001);
//   connectAuthEmulator(auth, "http://localhost:9099");
//   // connectStorageEmulator(storage, 'localhost', 9199);
// }

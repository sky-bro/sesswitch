/// <reference types="@vicinae/api">

/*
 * This file is auto-generated from the extension's manifest.
 * Do not modify manually. Instead, update the `package.json` file.
 */

type ExtensionPreferences = {}

declare type Preferences = ExtensionPreferences

declare namespace Preferences {
	/** Command: AI Sessions */
	export type AiSessions = ExtensionPreferences & {
		/** Sesswitch Binary - Absolute path to the sesswitch executable */
		"binaryPath": string;
	}
}

declare namespace Arguments {
	/** Command: AI Sessions */
	export type AiSessions = {}
}

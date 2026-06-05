// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

import { setupWorker } from "msw/browser";
import { handlers } from "./handlers";

export const worker = setupWorker(...handlers);

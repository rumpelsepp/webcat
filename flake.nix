# SPDX-FileCopyrightText: Stefan Tatschner <stefan@rumpelsepp.org>
#
# SPDX-License-Identifier: MPL-2.0

{
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs";
  };

  outputs = { self, nixpkgs }:
    with import nixpkgs { system = "x86_64-linux"; };
    let pkgs = nixpkgs.legacyPackages.x86_64-linux;
    in {
      devShell.x86_64-linux = pkgs.mkShell {
        buildInputs = with pkgs; [
          go
          gnumake
          gopls
          gotools
          reuse
        ];
      };
      formatter.x86_64-linux = pkgs.nixpkgs-fmt;
    };
}

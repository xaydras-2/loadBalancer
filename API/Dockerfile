# Use the official .NET SDK image to build the app
FROM mcr.microsoft.com/dotnet/sdk:9.0 AS build
WORKDIR /app

# Copy csproj and restore as distinct layers
COPY *.csproj ./
RUN dotnet restore

# Copy the rest of the source code
COPY . ./

# Build the application
RUN dotnet publish -c Release -o out

# Use the official .NET runtime image for running the app
FROM mcr.microsoft.com/dotnet/runtime:9.0
WORKDIR /app
COPY --from=build /app/out ./

# Expose the port your app listens on
EXPOSE 8080

ENV DOTNET_SYSTEM_NET_HTTP_USESOCKETSHTTPHANDLER=0


# Run the application
ENTRYPOINT ["dotnet", "API.dll"]